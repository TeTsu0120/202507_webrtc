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
		log.Printf("Client %p disconnected\n", c)
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error from client %p: %v\n", c, err)
			break
		}
		log.Printf("Received message from client %p: %s\n", c, message)

		// 他のクライアントへ転送
		clientsMu.Lock()
		for cli := range clients {
			if cli != c {
				select {
				case cli.send <- message:
					log.Printf("Forwarded message from client %p to client %p\n", c, cli)
				default:
					log.Printf("Send channel full, dropping message for client %p\n", cli)
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
			log.Printf("Write error to client %p: %v\n", c, err)
			return
		}
		log.Printf("Sent message to client %p: %s\n", c, message)
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
