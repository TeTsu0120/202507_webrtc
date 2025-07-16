// package main

// import (
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"math/rand"
// 	"net/url"
// 	"os"
// 	"os/signal"
// 	"sync"
// 	"syscall"
// 	"time"

// 	"github.com/gorilla/websocket"
// 	"github.com/notedit/gstreamer-go"
// 	"github.com/pion/webrtc/v4"
// 	"github.com/pion/webrtc/v4/pkg/media"
// )

// func must(err error) {
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

// type Message struct {
// 	Type             string  `json:"type,omitempty"`
// 	SDP              string  `json:"sdp,omitempty"`
// 	Candidate        string  `json:"candidate,omitempty"`
// 	SDPMid           *string `json:"sdpMid,omitempty"`
// 	SDPMLineIndex    *uint16 `json:"sdpMLineIndex,omitempty"`
// 	UsernameFragment string  `json:"usernameFragment,omitempty"`
// }

// func startGStreamerPush() (*gstreamer.Pipeline, *gstreamer.Element, error) {
	
// 	// Go → appsrc → エンコード → rtpvp8pay（WebRTCで送信用RTP）→ sink
// 	pipelineStr := "appsrc name=mysource format=time is-live=true do-timestamp=true ! " +
// 		"videoconvert ! vp8enc deadline=1 ! rtpvp8pay ! fakesink"

// 	fmt.Println("start startGStreamerPush pipelineStr=:", pipelineStr)

// 	pipeline, err := gstreamer.New(pipelineStr)
// 	if err != nil {
// 		fmt.Println("gstreamer.New err:", err)
// 		return nil, nil, err
// 	}

// 	appsrc := pipeline.FindElement("mysource")
// 	if appsrc == nil {
// 		fmt.Println("pipeline.FindElement:", err)
// 		return nil, nil, fmt.Errorf("appsrc not found")
// 	}

// 	appsrc.SetCap("video/x-raw,format=RGB,width=640,height=480")

// 	return pipeline, appsrc, nil
// }

// func generateDummyFrame(width, height int) []byte {
// 	return make([]byte, width*height*3) // RGB dummy frame
// }

// func main() {
// 	u := url.URL{Scheme: "ws", Host: "signaling-server:8080", Path: "/ws"}
// 	fmt.Println("Connecting to signaling server:", u.String())

// 	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	must(err)
// 	defer conn.Close()
// 	fmt.Println("Connected to signaling server")

// 	var wsWriteMutex sync.Mutex

// 	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
// 		ICEServers: []webrtc.ICEServer{},
// 	})
// 	must(err)

// 	videoTrack, err := webrtc.NewTrackLocalStaticSample(
// 		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
// 		fmt.Sprintf("video-%d", rand.Intn(999999)), "pion",
// 	)
// 	must(err)

// 	_, err = peerConnection.AddTrack(videoTrack)
// 	must(err)

// 	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
// 		if c == nil {
// 			return
// 		}
// 		payload, _ := json.Marshal(c.ToJSON())

// 		wsWriteMutex.Lock()
// 		defer wsWriteMutex.Unlock()
// 		_ = conn.WriteMessage(websocket.TextMessage, payload)
// 	})

// 	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
// 		fmt.Println("Connection State changed to:", state)
// 	})

// 	fmt.Println("Waiting for SDP offer from signaling server...")

// 	go func() {
// 		for {
// 			_, message, err := conn.ReadMessage()
// 			must(err)

// 			var msg Message
// 			if err := json.Unmarshal(message, &msg); err != nil {
// 				fmt.Println("⚠️ Failed to parse incoming JSON:", err)
// 				continue
// 			}

// 			if msg.Type == "offer" {
// 				fmt.Println("📥 Received SDP offer")

// 				offer := webrtc.SessionDescription{
// 					Type: webrtc.SDPTypeOffer,
// 					SDP:  msg.SDP,
// 				}
// 				must(peerConnection.SetRemoteDescription(offer))
// 				fmt.Println("✅ Set remote description")

// 				answer, err := peerConnection.CreateAnswer(nil)
// 				must(err)
// 				must(peerConnection.SetLocalDescription(answer))
// 				fmt.Println("📤 Created and set local SDP answer")

// 				answerBytes, _ := json.Marshal(peerConnection.LocalDescription())

// 				wsWriteMutex.Lock()
// 				_ = conn.WriteMessage(websocket.TextMessage, answerBytes)
// 				wsWriteMutex.Unlock()

// 				fmt.Println("✅ Sent SDP answer")

// 				// 映像配信スタート
// 				pipeline, appsrc, err := startGStreamerPush()
// 				must(err)
// 				pipeline.Start()

// 				go func() {
// 					ticker := time.NewTicker(time.Second / 30)
// 					defer ticker.Stop()

// 					for range ticker.C {
// 						frame := generateDummyFrame(640, 480)

// 						appsrc.Push(frame) // ← 修正（返り値不要）

// 						err := videoTrack.WriteSample(media.Sample{ // ← 修正
// 							Data:     frame,
// 							Duration: time.Second / 30,
// 						})
// 						if err != nil {
// 							log.Println("Failed to write sample:", err)
// 						}
// 					}
// 				}()
// 			} else if msg.Candidate != "" {
// 				fmt.Println("📥 Received ICE candidate")
// 				ice := webrtc.ICECandidateInit{
// 					Candidate:     msg.Candidate,
// 					SDPMid:        msg.SDPMid,
// 					SDPMLineIndex: msg.SDPMLineIndex,
// 				}
// 				_ = peerConnection.AddICECandidate(ice)
// 			}
// 		}
// 	}()

// 	// CTRL+Cで終了待ち
// 	sigs := make(chan os.Signal, 1)
// 	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
// 	<-sigs

// 	fmt.Println("Sender exiting")
// }

