package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
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

func main() {
	// Connect to signaling server
	u := url.URL{Scheme: "ws", Host: "signaling-server:8080", Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	must(err)
	defer conn.Close()

	// Create WebRTC peer connection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	must(err)

	// Create video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video", "pion")
	must(err)
	_, err = peerConnection.AddTrack(videoTrack)
	must(err)

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		payload, _ := json.Marshal(c.ToJSON())
		conn.WriteMessage(websocket.TextMessage, payload)
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Println("Connection State:", state)
	})

	// Read SDP offer
	_, message, err := conn.ReadMessage()
	must(err)

	var offer webrtc.SessionDescription
	must(json.Unmarshal(message, &offer))
	must(peerConnection.SetRemoteDescription(offer))

	// Create and send SDP answer
	answer, err := peerConnection.CreateAnswer(nil)
	must(err)
	must(peerConnection.SetLocalDescription(answer))

	answerBytes, _ := json.Marshal(answer)
	conn.WriteMessage(websocket.TextMessage, answerBytes)

	// Start sending dummy video frames
	ticker := time.NewTicker(33 * time.Millisecond) // ~30fps
	go func() {
		for range ticker.C {
			_ = videoTrack.WriteSample(media.Sample{
				Data:     []byte{0x00, 0x00, 0x01, 0x09, 0x10},
				Duration: time.Second / 30,
			})
		}
	}()

	// Wait for CTRL+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Println("Sender exiting")
}
