package main

import (
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
		log.Println("Client disconnected")
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		log.Printf("Received message from client: %s\n", message)

		// 他のクライアントへ転送
		clientsMu.Lock()
		for cli := range clients {
			if cli != c {
				select {
				case cli.send <- message:
					log.Println("Forwarded message to another client")
				default:
					log.Println("Send channel full, dropping message")
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
			return
		}
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Println("Write error:", err)
			return
		}
		log.Printf("Sent message to client: %s\n", message)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	log.Println("New client connected")

	c := &client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	clientsMu.Lock()
	clients[c] = true
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
