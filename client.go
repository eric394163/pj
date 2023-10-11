package main

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type client struct {
	socket *websocket.Conn
	send   chan []byte
	room   *room
	id     string
}

func (c *client) read() {
	defer c.socket.Close()
	for {
		_, msg, err := c.socket.ReadMessage()
		if err != nil {
			return
		}

		var incomingData map[string]interface{}
		err = json.Unmarshal(msg, &incomingData)
		if err != nil {
			log.Printf("마샬링 에러: %v", err)
			return
		}

		log.Printf("Received  message: %v", incomingData)
		processedMessage, err := json.Marshal(incomingData) // 다시 JSON으로 변환
		if err != nil {
			log.Printf("마샬링 에러: %v", err)
			return
		}

		c.room.forward <- processedMessage
	}
}

func (c *client) write() {
	defer c.socket.Close()
	for msg := range c.send {
		err := c.socket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
		}
	}
}
