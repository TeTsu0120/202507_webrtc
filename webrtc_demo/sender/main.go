package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Message は signaling 経由で送られてくるすべての JSON に対応する構造体
type Message struct {
	Type           string  `json:"type,omitempty"`
	SDP            string  `json:"sdp,omitempty"`
	Candidate      string  `json:"candidate,omitempty"`
	SDPMid         *string `json:"sdpMid,omitempty"`
	SDPMLineIndex  *uint16 `json:"sdpMLineIndex,omitempty"`
	UsernameFragment string `json:"usernameFragment,omitempty"`
}

func main() {
	u := url.URL{Scheme: "ws", Host: "signaling-server:8080", Path: "/ws"}
	fmt.Println("Connecting to signaling server:", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	must(err)
	defer conn.Close()
	fmt.Println("Connected to signaling server")

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	must(err)

	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video", "pion")
	must(err)

	_, err = peerConnection.AddTrack(videoTrack)
	must(err)

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			fmt.Println("ICE gathering complete")
			return
		}
		payload, _ := json.Marshal(c.ToJSON())
		fmt.Println("Sending ICE candidate:", string(payload))
		err := conn.WriteMessage(websocket.TextMessage, payload)
		if err != nil {
			fmt.Println("Error sending ICE candidate:", err)
		}
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Println("Connection State changed to:", state)
	})

	fmt.Println("Waiting for SDP offer from signaling server...")

	// SDP offer と ICE candidate を区別して処理
	for {
		_, message, err := conn.ReadMessage()
		must(err)

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			fmt.Println("⚠️ Failed to parse incoming JSON:", err)
			continue
		}

		if msg.Type == "offer" {
			fmt.Println("📥 Received SDP offer")

			offer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeOffer,
				SDP:  msg.SDP,
			}
			must(peerConnection.SetRemoteDescription(offer))
			fmt.Println("✅ Set remote description")

			answer, err := peerConnection.CreateAnswer(nil)
			must(err)
			must(peerConnection.SetLocalDescription(answer))
			fmt.Println("📤 Created and set local SDP answer")

			answerBytes, _ := json.Marshal(peerConnection.LocalDescription())
			err = conn.WriteMessage(websocket.TextMessage, answerBytes)
			if err != nil {
				fmt.Println("Error sending SDP answer:", err)
			} else {
				fmt.Println("✅ Sent SDP answer")
			}
		} else if msg.Candidate != "" {
			fmt.Println("📥 Received ICE candidate")

			ice := webrtc.ICECandidateInit{
				Candidate:     msg.Candidate,
				SDPMid:        msg.SDPMid,
				SDPMLineIndex: msg.SDPMLineIndex,
			}
			err := peerConnection.AddICECandidate(ice)
			if err != nil {
				fmt.Println("⚠️ Error adding ICE candidate:", err)
			} else {
				fmt.Println("✅ Added ICE candidate")
			}
		} else {
			fmt.Println("⚠️ Unknown message format:", string(message))
		}
	}

	// Start sending dummy video frames (この部分は別 goroutine にするなら main の外に移す)
	ticker := time.NewTicker(33 * time.Millisecond) // ~30fps
	go func() {
		for range ticker.C {
			err := videoTrack.WriteSample(media.Sample{
				Data:     []byte{0x00, 0x00, 0x01, 0x09, 0x10},
				Duration: time.Second / 30,
			})
			if err != nil {
				fmt.Println("Error writing video sample:", err)
			}
		}
	}()

	// Wait for CTRL+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Println("Sender exiting")
}
