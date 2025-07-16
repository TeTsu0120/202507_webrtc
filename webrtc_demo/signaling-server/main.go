package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

var (
	clients   = make(map[*client]bool)
	clientsMu sync.Mutex
)

func (c *client) readPump() {
	defer func() {
		clientsMu.Lock()
		delete(clients, c)
		clientsMu.Unlock()
		c.conn.Close()
		log.Printf("Client %p disconnected\n", c)
	}()

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error from client %p: %v\n", c, err)
			break
		}

		log.Printf("[Client %p] Received raw message (type %d): %s\n", c, messageType, message)

		// テキストメッセージのみ処理する
		if messageType != websocket.TextMessage {
			log.Printf("[Client %p] Ignoring non-text message\n", c)
			continue
		}

		// JSONパース
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("[Client %p] Invalid JSON: %v\n", c, err)
			continue
		}
		log.Printf("[Client %p] Parsed JSON message: %+v\n", c, msg)

		t, hasType := msg["type"]
		if hasType {
			log.Printf("[Client %p] 'type' field present: %v (type %T)\n", c, t, t)

			if num, ok := t.(float64); ok {
				switch num {
				case 0:
					msg["type"] = "offer"
				case 1:
					msg["type"] = "pranswer"
				case 2:
					msg["type"] = "answer"
				case 3:
					msg["type"] = "rollback"
				default:
					// そのまま
				}
				log.Printf("[Client %p] Converted 'type' field to string: %v\n", c, msg["type"])
			}
		} else {
			log.Printf("[Client %p] No 'type' field in message\n", c)
		}

		// 再エンコード
		fixedMsg, err := json.Marshal(msg)
		if err != nil {
			log.Printf("[Client %p] JSON marshal error: %v\n", c, err)
			continue
		}
		log.Printf("[Client %p] Forwarding JSON message: %s\n", c, fixedMsg)

		// 他のクライアントへ転送
		clientsMu.Lock()
		for cli := range clients {
			if cli != c {
				select {
				case cli.send <- fixedMsg:
					log.Printf("[Client %p] Forwarded message to client %p\n", c, cli)
				default:
					log.Printf("[Client %p] Send channel full, dropping message for client %p\n", c, cli)
				}
			}
		}
		clientsMu.Unlock()
	}
}

func (c *client) writePump() {
	defer c.conn.Close()
	for {
		message, ok := <-c.send
		if !ok {
			// チャンネルが閉じられたら終了
			log.Printf("Send channel closed for client %p\n", c)
			return
		}
		log.Printf("[Client %p] Sending message: %s\n", c, message)
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Write error to client %p: %v\n", c, err)
			return
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	clientsMu.Lock()
	clients[c] = true
	log.Printf("New client connected: %p, total clients: %d\n", c, len(clients))
	clientsMu.Unlock()

	go c.writePump()
	c.readPump()
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Signaling server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
