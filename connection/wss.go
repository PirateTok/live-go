package connection

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/PirateTok/live-go/events"
	tthttp "github.com/PirateTok/live-go/http"
	pb "github.com/PirateTok/live-go/proto"
	"google.golang.org/protobuf/proto"
)

const (
	defaultHeartbeatInterval = 10 * time.Second
	readBufferSize           = 65536
)

// DeviceBlockedError is returned when the WSS handshake responds with
// Handshake-Msg: DEVICE_BLOCKED, meaning the ttwid was flagged.
type DeviceBlockedError struct{}

func (e *DeviceBlockedError) Error() string {
	return "device blocked — ttwid was flagged, fetch a fresh one"
}

// RunWebSocket connects to the TikTok Live WSS endpoint and streams events.
// The userAgent parameter sets the User-Agent header for the WSS handshake.
// The cookieHeader is the full Cookie header value (e.g. "ttwid=xxx; sessionid=yyy").
// Pass acceptLanguage for locale-aware header (e.g. "ro-RO,ro;q=0.9"). Empty = auto-detect.
func RunWebSocket(ctx context.Context, wssURL string, cookieHeader string, userAgent string, roomID string, staleTimeout time.Duration, acceptLanguage string, eventCh chan<- events.Event) error {
	if acceptLanguage == "" {
		lang, reg := tthttp.SystemLocale()
		acceptLanguage = fmt.Sprintf("%s-%s,%s;q=0.9", lang, reg, lang)
	}
	header := http.Header{
		"User-Agent":      {userAgent},
		"Cookie":          {cookieHeader},
		"Origin":          {"https://www.tiktok.com"},
		"Referer":         {"https://www.tiktok.com/"},
		"Accept-Language": {acceptLanguage},
		"Accept-Encoding": {"gzip, deflate"},
		"Cache-Control":   {"no-cache"},
	}

	// Capture Handshake-Msg from non-101 error responses to detect DEVICE_BLOCKED.
	var handshakeMsg string
	dialer := ws.Dialer{
		Header: ws.HandshakeHeaderHTTP(header),
		OnStatusError: func(status int, reason []byte, resp io.Reader) {
			httpResp, err := http.ReadResponse(bufio.NewReader(resp), nil)
			if err != nil {
				return
			}
			defer httpResp.Body.Close()
			if val := httpResp.Header.Get("Handshake-Msg"); val != "" {
				handshakeMsg = val
			}
		},
	}

	conn, br, _, err := dialer.Dial(ctx, wssURL)
	if err != nil {
		if strings.EqualFold(handshakeMsg, "DEVICE_BLOCKED") {
			return &DeviceBlockedError{}
		}
		return fmt.Errorf("wss dial: %w", err)
	}
	if br != nil {
		ws.PutReader(br)
	}
	defer conn.Close()

	// send heartbeat + enter room
	hb, err := buildHeartbeat(roomID)
	if err != nil {
		return err
	}
	if err := wsutil.WriteClientBinary(conn, hb); err != nil {
		return fmt.Errorf("wss send heartbeat: %w", err)
	}

	enter, err := buildEnterRoom(roomID)
	if err != nil {
		return err
	}
	if err := wsutil.WriteClientBinary(conn, enter); err != nil {
		return fmt.Errorf("wss send enter room: %w", err)
	}

	// heartbeat goroutine
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(defaultHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hbBytes, err := buildHeartbeat(roomID)
				if err != nil {
					log.Printf("heartbeat build error: %v", err)
					return
				}
				if err := wsutil.WriteClientBinary(conn, hbBytes); err != nil {
					log.Printf("heartbeat send error: %v", err)
					return
				}
			}
		}
	}()

	err = readLoop(ctx, conn, roomID, staleTimeout, eventCh)

	// No disconnect emit — client owns lifecycle
	<-heartbeatDone
	return err
}

func readLoop(ctx context.Context, conn net.Conn, roomID string, staleTimeout time.Duration, eventCh chan<- events.Event) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := conn.SetReadDeadline(time.Now().Add(staleTimeout)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		msgs, err := wsutil.ReadServerMessage(conn, nil)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("stale: no data for %v", staleTimeout)
				return nil
			}
			return fmt.Errorf("wss read: %w", err)
		}

		for _, msg := range msgs {
			if msg.OpCode == ws.OpBinary {
				if err := processFrame(msg.Payload, conn, eventCh); err != nil {
					log.Printf("frame error: %v", err)
				}
			}
		}
	}
}

func processFrame(data []byte, conn net.Conn, eventCh chan<- events.Event) error {
	frame := &pb.WebcastPushFrame{}
	if err := proto.Unmarshal(data, frame); err != nil {
		return fmt.Errorf("unmarshal frame: %w", err)
	}

	switch frame.PayloadType {
	case "msg":
		decompressed, err := decompressIfGzipped(frame.Payload)
		if err != nil {
			return err
		}
		response := &pb.WebcastResponse{}
		if err := proto.Unmarshal(decompressed, response); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}

		if response.NeedsAck && len(response.InternalExt) > 0 {
			ack, err := buildAck(frame.LogId, response.InternalExt)
			if err == nil {
				if ackErr := wsutil.WriteClientBinary(conn, ack); ackErr != nil {
					log.Printf("ack send error: %v", ackErr)
				}
			}
		}

		for _, msg := range response.Messages {
			evts := events.Decode(msg.Method, msg.Payload)
			for _, evt := range evts {
				select {
				case eventCh <- evt:
				default:
				}
			}
		}

	case "im_enter_room_resp":
		// room entry confirmed
	case "hb":
		// heartbeat response
	}

	return nil
}
