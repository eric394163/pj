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
			log.Printf("JSON Unmarshal error: %v", err)
			return
		}

		// 여기서 incomingData는 map입니다.
		log.Printf("Received JSON message: %v", incomingData)
		processedMessage, err := json.Marshal(incomingData) // 다시 JSON으로 변환
		if err != nil {
			log.Printf("JSON Marshal error: %v", err)
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
