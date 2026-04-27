package tunnel

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ChannelControl = 0x00
	ChannelData   = 0x01

	DefaultControlPort = 9009
	DefaultDataPort    = 9010
)

var dialer = websocket.Dialer{
	NetDialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
}

type Tunnel struct {
	wsURL      string
	wsConn     *websocket.Conn
	ctrlLis    net.Listener
	dataLis    net.Listener
	cancel     context.CancelFunc
	sessionCtx context.Context
}

func Start(ctx context.Context, relayHost string) (*Tunnel, error) {
	url := fmt.Sprintf("wss://%s/relay", relayHost)

	wsConn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial to %s: %w", url, err)
	}

	ctrlLis, err := net.Listen("tcp", "127.0.0.1:9009")
	if err != nil {
		wsConn.Close()
		return nil, fmt.Errorf("listen on 9009: %w", err)
	}
	dataLis, err := net.Listen("tcp", "127.0.0.1:9010")
	if err != nil {
		ctrlLis.Close()
		wsConn.Close()
		return nil, fmt.Errorf("listen on 9010: %w", err)
	}

	tunnelCtx, cancel := context.WithCancel(context.Background())

	t := &Tunnel{
		wsURL:      url,
		wsConn:     wsConn,
		ctrlLis:    ctrlLis,
		dataLis:    dataLis,
		cancel:     cancel,
		sessionCtx: tunnelCtx,
	}

	go t.acceptLoop(ctrlLis, ChannelControl)
	go t.acceptLoop(dataLis, ChannelData)
	go t.wsToListeners()

	return t, nil
}

func (t *Tunnel) Close() error {
	t.cancel()

	var errs []error

	if t.ctrlLis != nil {
		if err := t.ctrlLis.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if t.dataLis != nil {
		if err := t.dataLis.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if t.wsConn != nil {
		if err := t.wsConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (t *Tunnel) acceptLoop(lis net.Listener, channel byte) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			if t.sessionCtx.Err() != nil {
				return
			}
			netErr, ok := err.(net.Error)
			if ok && netErr.Temporary() {
				continue
			}
			return
		}

		go t.handleAccept(conn, channel)
	}
}

func (t *Tunnel) handleAccept(conn net.Conn, channel byte) {
	defer conn.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
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

		if err := t.wsConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			return
		}
	}
}

func (t *Tunnel) wsToListeners() {
	defer t.cancel()

	for {
		msgType, payload, err := t.wsConn.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		if len(payload) < 1 {
			continue
		}

		var lis net.Listener
		switch payload[0] {
		case ChannelControl:
			lis = t.ctrlLis
		case ChannelData:
			lis = t.dataLis
		default:
			continue
		}

		if lis == nil {
			continue
		}

		t.forwardPayload(payload[1:], lis)
	}
}

func (t *Tunnel) forwardPayload(payload []byte, lis net.Listener) {
	conn, err := lis.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	_, err = conn.Write(payload)
	if err != nil {
		return
	}
}

func CheckWSRelay(ctx context.Context, relayHost string, timeout time.Duration) error {
	url := fmt.Sprintf("https://%s/relay", relayHost)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusSwitchingProtocols {
		return nil
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

func init() {
	log.SetFlags(0)
}