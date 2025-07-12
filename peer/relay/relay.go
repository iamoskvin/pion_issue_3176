package relay

import (
	"encoding/json"
	"log"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

type Message struct {
	Cmd  int    `json:"cmd"`
	Data string `json:"data"`
}

type OnMessageFunc func(msg Message)

type RelayClient struct {
	conn       *websocket.Conn
	onMessage  OnMessageFunc
	sendCh     chan Message
	closeOnce  sync.Once
	closed     chan struct{}
	peerID     string
	writeMutex sync.Mutex
}

// NewRelayClient connects to the signaling server at the given address (e.g., "localhost:8081"),
// registers the peerID, and starts listening for messages.
func NewRelayClient(serverAddr string, peerID string, onMessage OnMessageFunc) (*RelayClient, error) {
	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	rc := &RelayClient{
		conn:      conn,
		onMessage: onMessage,
		sendCh:    make(chan Message, 16),
		closed:    make(chan struct{}),
		peerID:    peerID,
	}

	// Register the peer
	registerMsg := Message{
		Cmd: 4,
		Data: mustJSON(map[string]string{
			"peer_id": peerID,
		}),
	}
	rc.sendCh <- registerMsg

	// Start read and write loops
	go rc.readLoop()
	go rc.writeLoop()

	return rc, nil
}

func (rc *RelayClient) Send(msg Message) {
	select {
	case rc.sendCh <- msg:
	case <-rc.closed:
		log.Println("Attempted to send on closed relay client")
	}
}

func (rc *RelayClient) Close() {
	rc.closeOnce.Do(func() {
		close(rc.closed)
		rc.conn.Close()
	})
}

func (rc *RelayClient) readLoop() {
	for {
		select {
		case <-rc.closed:
			return
		default:
			var msg Message
			err := rc.conn.ReadJSON(&msg)
			if err != nil {
				log.Println("RelayClient read error:", err)
				rc.Close()
				return
			}
			if rc.onMessage != nil {
				rc.onMessage(msg)
			}
		}
	}
}

func (rc *RelayClient) writeLoop() {
	for {
		select {
		case msg := <-rc.sendCh:
			rc.writeMutex.Lock()
			err := rc.conn.WriteJSON(msg)
			rc.writeMutex.Unlock()
			if err != nil {
				log.Println("RelayClient write error:", err)
				rc.Close()
				return
			}
		case <-rc.closed:
			return
		}
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
