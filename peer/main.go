package main

import (
	"encoding/json"
	"fmt"
	"issue/peer/relay"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v4"
)

type Envelope struct {
	FromPeerID string `json:"from_peer_id"`
	ToPeerID   string `json:"to_peer_id"`
	EventName  string `json:"event_name"`
	Data       string `json:"data"`
}

var remotePeerID string
var peerID string
var relayClient *relay.RelayClient
var peerConn *webrtc.PeerConnection

func main() {
	var err error
	err = godotenv.Load() // by default loads .env
	if err != nil {
		log.Println(".env not found, using existing environment")
	}
	filterHostCand := os.Getenv("FILTER_HOST_CAND")
	if filterHostCand == "" {
		log.Fatalf("Missing required env var FILTER_HOST")
	}
	signalingPort := os.Getenv("SIGNALING_PORT")
	if signalingPort == "" {
		signalingPort = "8081"
	}

	peerID = "Z7H0LOIW6O2LH4U5"
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}

	var settingEngine webrtc.SettingEngine
	settingEngine.SetInterfaceFilter(func(name string) bool {
		if filterHostCand == "true" {
			return false
		}
		return true
	})
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
	})
	settingEngine.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)

	loggingFactory := logging.NewDefaultLoggerFactory()
	loggingFactory.DefaultLogLevel.Set(logging.LogLevelTrace)
	settingEngine.LoggerFactory = loggingFactory

	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))
	peerConn, err = api.NewPeerConnection(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Peer created", "signalingPort: ", signalingPort)
	peerConn.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Println("📨 Got incoming DataChannel")
		dc.OnOpen(func() {
			log.Println("✅ DataChannel opened (answerer)")
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			log.Printf("Message from peer: %s\n", string(msg.Data))
		})

	})

	relayClient, err = relay.NewRelayClient("localhost:"+signalingPort, peerID, handleRelayMsg)
	if err != nil {
		log.Fatal(err)
	}
	defer relayClient.Close()

	peerConn.OnICECandidate(func(cand *webrtc.ICECandidate) {
		if cand == nil || remotePeerID == "" {
			return
		}
		jsonCand, err := json.Marshal(cand.ToJSON())
		if err != nil {
			log.Println("Failed to marshal ICE candidate:", err)
			return
		}
		// if strings.Contains(string(jsonCand), "host") {
		// 	return
		// }
		envelope := Envelope{
			FromPeerID: peerID,
			ToPeerID:   remotePeerID,
			EventName:  "icecandidate",
			Data:       string(jsonCand),
		}
		envBytes, err := json.Marshal(envelope)
		if err != nil {
			log.Println("Failed to marshal envelope for ICE candidate:", err)
			return
		}

		msg := relay.Message{
			Cmd:  2,
			Data: string(envBytes),
		}
		relayClient.Send(msg)

	})

	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("PeerConnection state:", state)
	})

	select {}
}

func handleRelayMsg(msg relay.Message) {
	fmt.Println("Got message from signaling:", msg)

	if msg.Cmd != 2 {
		return
	}
	var env Envelope
	if err := json.Unmarshal([]byte(msg.Data), &env); err != nil {
		log.Println("Failed to unmarshal envelope:", err)
		return
	}
	remotePeerID = env.FromPeerID

	switch env.EventName {
	case "offer":
		var offer webrtc.SessionDescription
		if err := json.Unmarshal([]byte(env.Data), &offer); err != nil {
			log.Println("Failed to unmarshal offer:", err)
			return
		}
		if err := peerConn.SetRemoteDescription(offer); err != nil {
			log.Println("SetRemoteDescription failed:", err)
			return
		}

		answer, err := peerConn.CreateAnswer(nil)
		if err != nil {
			log.Println("CreateAnswer failed:", err)
			return
		}
		if err := peerConn.SetLocalDescription(answer); err != nil {
			log.Println("SetLocalDescription failed:", err)
			return
		}
		answerBytes, err := json.Marshal(answer)
		if err != nil {
			log.Println("Failed to marshal answer:", err)
			return
		}

		envelope := Envelope{
			FromPeerID: peerID,
			ToPeerID:   remotePeerID,
			EventName:  "answer",
			Data:       string(answerBytes),
		}
		envBytes, err := json.Marshal(envelope)
		if err != nil {
			log.Println("Failed to marshal envelope for answer:", err)
			return
		}

		msg := relay.Message{
			Cmd:  2,
			Data: string(envBytes),
		}
		relayClient.Send(msg)

	case "icecandidate":
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal([]byte(env.Data), &candidate); err != nil {
			log.Println("Failed to parse ICE candidate:", err)
			return
		}
		if err := peerConn.AddICECandidate(candidate); err != nil {
			log.Println("AddICECandidate failed:", err)
		}
	}
}
