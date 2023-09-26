package main

import (
	"encoding/json"
	"fmt"
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

func (r *room) run() {
	for {
		select {
		case client := <-r.join:
			r.clients[client] = true
			r.onlineUsers[client.id] = true
			r.broadcastUserList()

			joinMessage := fmt.Sprintf("%s님이 채팅방에 접속하였습니다.", client.id)
			joinMessageJSON, _ := json.Marshal(map[string]interface{}{
				"type":    "system",
				"message": joinMessage,
			})
			for otherClient := range r.clients {
				otherClient.send <- joinMessageJSON
			}

		case client := <-r.leave:
			delete(r.clients, client)
			delete(r.onlineUsers, client.id)
			close(client.send)
			r.broadcastUserList()

		case msg := <-r.forward:
			for client := range r.clients {
				client.send <- msg
			}
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
