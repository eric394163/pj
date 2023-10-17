package main

import (
	"encoding/json"
	"fmt"
	"gopjex/dbcon"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type room struct {
	forward chan []byte
	join    chan *client
	leave   chan *client
	clients map[*client]bool

	onlineUsers map[string]bool
}

func newRoom() *room {
	return &room{

		forward: make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),

		onlineUsers: make(map[string]bool),
	}
}

func (r *room) broadcastUserList() {
	userList := make([]string, 0, len(r.onlineUsers))
	for user := range r.onlineUsers {
		userList = append(userList, user)
	}

	userListJSON, err := json.Marshal(map[string]interface{}{
		"type":  "userList",
		"users": userList,
	})

	if err != nil {
		log.Printf("사용자 목록 마샬링 오류: %v", err)
		return
	}

	for client := range r.clients {
		client.send <- userListJSON
	}
}

func saveChatMessageToDB(dbc *dbcon.DBConnection, roomName, userID string, message json.RawMessage) {
	// 큰따옴표 제거
	roomName = roomName[1 : len(roomName)-1]
	userID = userID[1 : len(userID)-1]

	sql := "INSERT INTO MAIN_Chat_Storage (roomName, userID, message) VALUES (?, ?, ?)"

	stmt, err := dbc.Conn.Prepare(sql)
	if err != nil {
		log.Fatalf("Failed to prepare SQL: %v", err)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(roomName, userID, message)
	if err != nil {
		log.Fatalf("Failed to execute SQL: %v", err)
	}
}

func getChatMessagesFromDB(dbc *dbcon.DBConnection, roomName string) []map[string]json.RawMessage {
	rows, err := dbc.Conn.Query("SELECT userID, message FROM MAIN_Chat_Storage WHERE roomName = ? ORDER BY timestamp ASC", roomName)
	if err != nil {
		log.Fatalf("Failed to query messages: %v", err)
		return nil
	}
	defer rows.Close()

	var messages []map[string]json.RawMessage
	for rows.Next() {
		var userID string
		var message json.RawMessage
		if err := rows.Scan(&userID, &message); err != nil {
			log.Fatalf("Failed to scan message: %v", err)
			continue
		}
		msg := map[string]json.RawMessage{
			"userID":  json.RawMessage(`"` + userID + `"`),
			"message": message,
		}
		messages = append(messages, msg)
	}

	//클라이언트로 저장된 메세지가 제대로 갔는지 확인하기
	log.Printf(" %d 메세지가 %s으로 갔음", len(messages), roomName)

	return messages
}

func (r *room) handleJoin(client *client, dbc *dbcon.DBConnection) {
	r.clients[client] = true
	r.onlineUsers[client.id] = true
	r.broadcastUserList()

	roomName := "main"
	storedMessages := getChatMessagesFromDB(dbc, roomName)

	storedMessagesJSON, err := json.Marshal(map[string]interface{}{
		"type":        "chatHistory",
		"chatHistory": storedMessages,
	})
	if err != nil {
		log.Printf("저장된 메세지 마샬링 에러: %v", err)
		return
	}
	client.send <- storedMessagesJSON

	joinMessage := fmt.Sprintf("%s님이 채팅방에 접속했습니다.", client.id)
	joinMessageJSON, err := json.Marshal(map[string]interface{}{
		"type":    "system",
		"message": joinMessage,
	})
	if err != nil {
		log.Printf("조인 메세지 마샬링 에러: %v", err)
		return
	}

	for otherClient := range r.clients {
		otherClient.send <- joinMessageJSON
	}
}

func (r *room) handleLeave(client *client) {
	delete(r.clients, client)
	delete(r.onlineUsers, client.id)
	close(client.send)
	r.broadcastUserList()
}

func (r *room) handleForward(msg []byte, dbc *dbcon.DBConnection) {
	var messageData map[string]json.RawMessage
	err := json.Unmarshal(msg, &messageData)
	if err != nil {
		log.Printf("메세지 마샬링 에러: %v", err)
		return
	}

	roomName := string(messageData["roomName"])
	userID := string(messageData["userID"])
	message := messageData["message"]
	saveChatMessageToDB(dbc, roomName, userID, message)

	for client := range r.clients {
		client.send <- msg
	}
}

func (r *room) run(dbc *dbcon.DBConnection) {

	if dbc == nil {
		log.Fatalf("Received nil database connection")
		return
	}

	for {
		select {
		case client := <-r.join:
			r.handleJoin(client, dbc)

		case client := <-r.leave:
			r.handleLeave(client)

		case msg := <-r.forward:
			r.handleForward(msg, dbc)

		}
	}
}

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{

	ReadBufferSize:  socketBufferSize,
	WriteBufferSize: socketBufferSize,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP:", err)
		return
	}

	authCookie, err := req.Cookie("auth")
	if err != nil {
		http.Error(w, "인증되지않음", http.StatusUnauthorized)
		return
	}

	userID := authCookie.Value

	client := &client{
		socket: socket,
		send:   make(chan []byte, messageBufferSize),
		room:   r,
		id:     userID,
	}
	r.join <- client
	defer func() { r.leave <- client }()
	go client.write()
	client.read()
}
