package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type Message struct {
	Cmd  int    `json:"cmd"`
	Data string `json:"data"`
}

type Envelope struct {
	FromPeerID string `json:"from_peer_id"`
	ToPeerID   string `json:"to_peer_id"`
	EventName  string `json:"event_name"`
	Data       string `json:"data"`
}

type Client struct {
	conn *websocket.Conn
	id   string
}

var (
	clients   = make(map[string]*Client)
	clientsMu sync.RWMutex
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func main() {
	err := godotenv.Load() // by default loads .env
	if err != nil {
		log.Println(".env not found, using existing environment")
	}
	signalingPort := os.Getenv("SIGNALING_PORT")
	if signalingPort == "" {
		signalingPort = "8081"
	}
	http.HandleFunc("/ws", handleWebSocket)
	log.Println("Listening on :" + signalingPort)
	log.Fatal(http.ListenAndServe(":"+signalingPort, nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	var clientID string

	for {
		var msg Message
		msgType, packet, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		fmt.Println("msgType", msgType, string(packet))
		json.Unmarshal(packet, &msg)

		switch msg.Cmd {
		case 2:
			// signaling
			var env Envelope
			if err := json.Unmarshal([]byte(msg.Data), &env); err != nil {
				log.Println("Invalid signaling payload:", err)
				continue
			}
			if clientID == "" {
				clientID = env.FromPeerID
				registerClient(clientID, conn)
			}
			if dest := getClient(env.ToPeerID); dest != nil {
				dest.conn.WriteJSON(msg)
			} else {
				log.Printf("Peer %s not found\n", env.ToPeerID)
			}
		case 4:
			var ping struct {
				PeerID string `json:"peer_id"`
			}
			if err := json.Unmarshal([]byte(msg.Data), &ping); err != nil {
				log.Println("Invalid ping:", err)
				continue
			}
			if clientID == "" {
				clientID = ping.PeerID
				registerClient(clientID, conn)
			}
		default:
			log.Printf("Custom cmd=%d from %s: %s\n", msg.Cmd, clientID, msg.Data)
		}
	}

	unregisterClient(clientID)
}

func registerClient(id string, conn *websocket.Conn) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clients[id] = &Client{conn: conn, id: id}
	log.Printf("Client %s registered\n", id)
}

func unregisterClient(id string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if id != "" {
		delete(clients, id)
		log.Printf("Client %s unregistered\n", id)
	}
}

func getClient(id string) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	return clients[id]
}
