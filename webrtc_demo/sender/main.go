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

func main() {
	u := url.URL{Scheme: "ws", Host: "signaling-server:8080", Path: "/ws"}
	fmt.Println("Connecting to signaling server:", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	must(err)
	defer conn.Close()
	fmt.Println("Connected to signaling server")

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
    ICEServers: []webrtc.ICEServer{
        {
            URLs: []string{"stun:stun.l.google.com:19302"},
        },
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
	_, message, err := conn.ReadMessage()
	must(err)
	fmt.Println("Received SDP offer:", string(message))

	var offer webrtc.SessionDescription
	must(json.Unmarshal(message, &offer))
	must(peerConnection.SetRemoteDescription(offer))
	fmt.Println("Set remote description")

	answer, err := peerConnection.CreateAnswer(nil)
	must(err)
	must(peerConnection.SetLocalDescription(answer))
	fmt.Println("Created and set local SDP answer")

	answerBytes, _ := json.Marshal(answer)
	err = conn.WriteMessage(websocket.TextMessage, answerBytes)
	if err != nil {
		fmt.Println("Error sending SDP answer:", err)
	} else {
		fmt.Println("Sent SDP answer")
	}

	// Start sending dummy video frames
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
