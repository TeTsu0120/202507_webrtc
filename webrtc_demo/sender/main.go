package main

import (
	"encoding/json"
	"log"
	"net/url"
	"sync"
	"time"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	gst "github.com/notedit/gstreamer-go"
)

type offerHandler struct {
	pc        *webrtc.PeerConnection
	iceCandCh chan webrtc.ICECandidateInit
	done      chan struct{}
	conn      *websocket.Conn
	writeMu   sync.Mutex // WebSocket書き込み排他用
}

func (h *offerHandler) sendJSON(v interface{}) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	msg, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return h.conn.WriteMessage(websocket.TextMessage, msg)
}

func main() {
	signalingURL := "ws://signaling-server:8080/ws"
	u, err := url.Parse(signalingURL)
	if err != nil {
		log.Fatalf("Invalid signaling URL: %v", err)
	}

	log.Printf("Connecting to signaling server: %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("Failed to connect to signaling server: %v", err)
	}
	defer conn.Close()
	log.Println("Connected to signaling server")

	var handler *offerHandler
	var mu sync.Mutex

	// handler未作成時に来るICE候補を一時保存するバッファ
	var pendingCandidates []webrtc.ICECandidateInit

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Fatalf("read error: %v", err)
		}
		log.Printf("Received message: %s", msg)

		var raw map[string]interface{}
		if err := json.Unmarshal(msg, &raw); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		// ICE candidate判定
		if _, ok := raw["candidate"]; ok {
			var iceCandidate webrtc.ICECandidateInit
			if err := json.Unmarshal(msg, &iceCandidate); err != nil {
				log.Printf("Failed to parse ICE candidate: %v", err)
				continue
			}
			mu.Lock()
			if handler != nil {
				select {
				case handler.iceCandCh <- iceCandidate:
					log.Println("ICE candidate sent to handler")
				default:
					log.Println("ICE candidate channel full, dropping")
				}
			} else {
				log.Println("No handler ready for ICE candidate, storing temporarily")
				pendingCandidates = append(pendingCandidates, iceCandidate)
			}
			mu.Unlock()
			continue
		}

		// SDP type の数値→文字列変換
		if t, ok := raw["type"].(float64); ok {
			switch int(t) {
			case 0:
				raw["type"] = "offer"
			case 1:
				raw["type"] = "pranswer"
			case 2:
				raw["type"] = "answer"
			case 3:
				raw["type"] = "rollback"
			}
		}

		fixedMsg, err := json.Marshal(raw)
		if err != nil {
			log.Printf("Failed to marshal fixed message: %v", err)
			continue
		}

		var offer webrtc.SessionDescription
		if err := json.Unmarshal(fixedMsg, &offer); err != nil {
			log.Printf("Not an SDP offer, skipping: %v", err)
			continue
		}

		log.Println("Offer received, handling in goroutine")

		mu.Lock()
		if handler != nil {
			close(handler.done)
		}
		handler = &offerHandler{
			iceCandCh: make(chan webrtc.ICECandidateInit, 10),
			done:      make(chan struct{}),
			conn:      conn,
		}

		// 保留していたICE候補をすべて渡す
		for _, c := range pendingCandidates {
			select {
			case handler.iceCandCh <- c:
				log.Println("Replaying stored ICE candidate to handler")
			default:
				log.Println("ICE candidate channel full when replaying, dropping")
			}
		}
		pendingCandidates = nil

		mu.Unlock()

		go func(h *offerHandler, sdp webrtc.SessionDescription) {
			err := h.handleOffer(sdp)
			if err != nil {
				log.Printf("handleOffer error: %v", err)
			}
			mu.Lock()
			if handler == h {
				handler = nil
			}
			mu.Unlock()
		}(handler, offer)
	}
}

func (h *offerHandler) handleOffer(offer webrtc.SessionDescription) error {
	log.Println("handleOffer started")

	mediaEngine := &webrtc.MediaEngine{}
	mediaEngine.RegisterDefaultCodecs()
	log.Println("MediaEngine initialized")

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	turnHost := os.Getenv("HOST_IP") // → "host.docker.internal"
	turnURL := fmt.Sprintf("turn:%s:3478", turnHost)

	pc, err := api.NewPeerConnection(webrtc.Configuration{ICEServers: []webrtc.ICEServer{
        {URLs: []string{"stun:stun.l.google.com:19302"}},
		{
      		// URLs:       []string{"turn:host.docker.internal:3478"},
			URLs:       []string{turnURL},
      		Username:   "testuser",
      		Credential: "testpass",
    	},
    },
	})
	if err != nil {
		return err
	}
	h.pc = pc
	log.Println("PeerConnection created")

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeVP8,
	}, "video", "pion")
	if err != nil {
		return err
	}
	log.Println("Video track created")

	_, err = pc.AddTrack(videoTrack)
	if err != nil {
		return err
	}
	log.Println("Video track added")

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			log.Println("ICE candidate gathering finished")
			return
		}
		log.Println("Sending ICE candidate")
		if err := h.sendJSON(c.ToJSON()); err != nil {
			log.Printf("Failed to send ICE candidate: %v", err)
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Connection State changed: %s", state.String())
	})

	log.Println("Setting RemoteDescription")
	if err := pc.SetRemoteDescription(offer); err != nil {
		return err
	}
	log.Println("RemoteDescription set")

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	log.Println("Answer created")

	if err := pc.SetLocalDescription(answer); err != nil {
		return err
	}
	log.Println("LocalDescription set")

	if err := h.sendJSON(answer); err != nil {
		log.Printf("Failed to send SDP answer: %v", err)
	}

	// ICE candidate受信ループ
	go func() {
		for {
			select {
			case ice := <-h.iceCandCh:
				log.Printf("Adding remote ICE candidate: %v", ice)
				if err := pc.AddICECandidate(ice); err != nil {
					log.Printf("Failed to add ICE candidate: %v", err)
				}
			case <-h.done:
				log.Println("ICE candidate handling stopped")
				return
			}
		}
	}()

	log.Println("Creating GStreamer pipeline")
	// pipelineStr := "videotestsrc is-live=true ! video/x-raw,format=I420,width=640,height=480,framerate=30/1 ! vp8enc deadline=1 ! appsink name=sink emit-signals=true sync=false max-buffers=5 drop=true"
	pipelineStr := `
		videotestsrc is-live=true 
		! video/x-raw,format=I420,width=640,height=480,framerate=30/1 
		! clockoverlay auto-resize=false time-format="%Y-%m-%d %H:%M:%S"
		! vp8enc deadline=1 
		! appsink name=sink emit-signals=true sync=false max-buffers=5 drop=true`

	pipeline, err := gst.New(pipelineStr)

	appsink := pipeline.FindElement("sink")
	if appsink == nil {
    return fmt.Errorf("appsink element not found")
	}
	out := appsink.Poll()

	pipeline.Start()
	log.Println("GStreamer pipeline started")

	go func() {
		for {
			buffer := <-out
			if buffer == nil {
				log.Println("Received nil buffer, exiting goroutine")
				return
        	}
			err := videoTrack.WriteSample(media.Sample{
				Data:     buffer,
				Duration: time.Second / 30,
			})
			if err != nil {
				log.Printf("WriteSample error: %v", err)
			}
		}
	}()

	return nil
}
