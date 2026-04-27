package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	channelControl = 0x00
	channelData    = 0x01
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9099", "listen address for qim-mux")
	relayControlPort := flag.String("relay-control", "9009", "local croc relay control port")
	relayDataPort := flag.String("relay-data", "9010", "local croc relay data port")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/relay", handleRelay(*relayControlPort, *relayDataPort))
	mux.HandleFunc("/health", handleHealth)

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	log.Printf("qim-mux listening on %s", *addr)
	log.Printf("relay targets: %s (control), %s (data)", *relayControlPort, *relayDataPort)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func handleRelay(relayControl, relayData string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		wsCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrlConn, err := net.Dial("tcp", "127.0.0.1:"+relayControl)
		if err != nil {
			log.Printf("dial control port %s failed: %v", relayControl, err)
			return
		}
		defer ctrlConn.Close()

		dataConn, err := net.Dial("tcp", "127.0.0.1:"+relayData)
		if err != nil {
			log.Printf("dial data port %s failed: %v", relayData, err)
			return
		}
		defer dataConn.Close()

		var wg sync.WaitGroup
		wg.Add(3)

		go func() {
			defer wg.Done()
			defer cancel()
			wsToTCP(wsCtx, conn, ctrlConn, dataConn)
		}()
		go func() {
			defer wg.Done()
			defer cancel()
			tcpToWS(wsCtx, conn, ctrlConn, channelControl)
		}()
		go func() {
			defer wg.Done()
			defer cancel()
			tcpToWS(wsCtx, conn, dataConn, channelData)
		}()

		wg.Wait()
	}
}

func wsToTCP(ctx context.Context, ws *websocket.Conn, ctrl, data net.Conn) {
	for {
		msgType, payload, err := ws.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		if len(payload) < 1 {
			continue
		}

		var target net.Conn
		switch payload[0] {
		case channelControl:
			target = ctrl
		case channelData:
			target = data
		default:
			continue
		}
		if _, err := target.Write(payload[1:]); err != nil {
			return
		}
	}
}

func tcpToWS(ctx context.Context, ws *websocket.Conn, src net.Conn, channel byte) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				return
			}
			return
		}
		frame := make([]byte, n+1)
		frame[0] = channel
		copy(frame[1:], buf[:n])
		if err := ws.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			return
		}
	}
}
