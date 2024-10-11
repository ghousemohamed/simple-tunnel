package client

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"github.com/gorilla/websocket"
	"net/url"
	"sync"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"time"
)

const (
	green  = "\033[32m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

// WebSocket opcodes
const (
	WebSocketContinuationFrame = 0
	WebSocketTextFrame         = 1
	WebSocketBinaryFrame       = 2
	WebSocketCloseFrame        = 8
	WebSocketPingFrame         = 9
	WebSocketPongFrame         = 10
)

type Client struct {
	httpPort string
	serverAddr string
	subdomain string
}

func init() {
	log.SetFlags(0)
}

func NewClient(httpPort string, serverAddr string, subdomain string) *Client {
	return &Client{
		httpPort: httpPort,
		serverAddr: serverAddr,
		subdomain: subdomain,
	}
}

func (c *Client) StartClient() error {
	tunnelURL := fmt.Sprintf("http://%s/_tunnel?subdomain=%s", c.serverAddr, c.subdomain)

	var conn net.Conn
	conn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	req, err := http.NewRequest("GET", tunnelURL, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	if err := req.Write(conn); err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		log.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}

	// Check if the connection has been upgraded
	if strings.ToLower(resp.Header.Get("Upgrade")) != "websocket" {
		log.Fatalf("Server did not upgrade to WebSocket")
	}

	log.Printf("Your site is now available at: https://%s.%s", c.subdomain, c.serverAddr)

	for {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			if err == io.EOF {
				log.Println("Tunnel closed by server")
				return nil
			}
			log.Printf("%sError reading request: %v%s", red, err, reset)
			continue
		}

		// Log the request in the desired format with color
		log.Printf("%s%s %s %s%s", green, req.Method, req.URL.Path, req.Proto, reset)

		if websocket.IsWebSocketUpgrade(req) {
			c.handleWebSocketRequest(conn, req)
		} else {
			handleHTTP(c, conn, req)
		}
	}
}

func handleHTTP(c *Client, conn net.Conn, req *http.Request) {
	localURL := fmt.Sprintf("http://localhost:%s%s", c.httpPort, req.URL.Path)
	if req.URL.RawQuery != "" {
		localURL += "?" + req.URL.RawQuery
	}

	localReq, err := http.NewRequest(req.Method, localURL, req.Body)
	if err != nil {
		log.Printf("Error creating local request: %v", err)
		return
	}

	localReq.Header = req.Header

	localResp, err := http.DefaultClient.Do(localReq)
	if err != nil {
		log.Printf("Error sending request to local server: %v", err)
		return
	}
	defer localResp.Body.Close()

	err = localResp.Write(conn)
	if err != nil {
		log.Printf("Error writing response to tunnel: %v", err)
	}
}

func (c *Client) handleWebSocketRequest(conn net.Conn, req *http.Request) {
	dialer := websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.Dial("tcp", fmt.Sprintf("localhost:%s", c.httpPort))
		},
	}

	u, err := url.Parse(fmt.Sprintf("ws://localhost:%s%s", c.httpPort, req.URL.Path))
	if err != nil {
		log.Printf("Failed to parse WebSocket URL: %v", err)
		return
	}

	u.RawQuery = req.URL.RawQuery

	header := make(http.Header)
	for k, v := range req.Header {
		switch k {
		case "Upgrade", "Connection", "Sec-Websocket-Key",
			 "Sec-Websocket-Version", "Sec-Websocket-Extensions",
			 "Sec-Websocket-Protocol":
		default:
			header[k] = v
		}
	}

	localWS, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		log.Printf("Failed to connect to local WebSocket server: %v", err)
		if resp != nil {
			log.Printf("Response status: %s", resp.Status)
		}
		return
	}
	defer localWS.Close()

	upgradeResp := &http.Response{
		Status:     "101 Switching Protocols",
		StatusCode: http.StatusSwitchingProtocols,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	upgradeResp.Header.Set("Upgrade", "websocket")
	upgradeResp.Header.Set("Connection", "Upgrade")
	upgradeResp.Header.Set("Sec-WebSocket-Accept", computeAccept(req.Header.Get("Sec-WebSocket-Key")))

	if err := upgradeResp.Write(conn); err != nil {
		log.Printf("Failed to send upgrade response: %v", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			messageType, p, err := localWS.ReadMessage()
			if err != nil {
				log.Printf("Error reading from local WebSocket: %v", err)
				return
			}
			log.Printf("Client received message from local WebSocket: %s", string(p))
			var wsMessageType int
			switch messageType {
			case websocket.TextMessage:
				wsMessageType = WebSocketTextFrame
			case websocket.BinaryMessage:
				wsMessageType = WebSocketBinaryFrame
			case websocket.CloseMessage:
				wsMessageType = WebSocketCloseFrame
			case websocket.PingMessage:
				wsMessageType = WebSocketPingFrame
			case websocket.PongMessage:
				wsMessageType = WebSocketPongFrame
			default:
				log.Printf("Unknown message type: %d", messageType)
				continue
			}
			if err := writeWebSocketMessage(conn, wsMessageType, p); err != nil {
				log.Printf("Error writing to tunnel: %v", err)
				return
			}
			log.Printf("Client forwarded message to tunnel: %s", string(p))
		}
	}()

	go func() {
		defer wg.Done()
		for {
			messageType, p, err := readWebSocketMessage(conn)
			if err != nil {
				log.Printf("Error reading from tunnel: %v", err)
				return
			}
			if messageType == 0 && p == nil {
				continue
			}
			log.Printf("Client received message from tunnel: %s", string(p))
			if err := localWS.WriteMessage(messageType, p); err != nil {
				log.Printf("Error writing to local WebSocket: %v", err)
				return
			}
		}
	}()

	wg.Wait()
}

func computeAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func writeWebSocketMessage(conn net.Conn, messageType int, payload []byte) error {
	// https://tools.ietf.org/html/rfc6455#section-5.2
	var header []byte
	if len(payload) < 126 {
		header = make([]byte, 2)
		header[1] = byte(len(payload))
	} else if len(payload) < 65536 {
		header = make([]byte, 4)
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(len(payload)))
	} else {
		header = make([]byte, 10)
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(len(payload)))
	}

	header[0] = byte(messageType) | 0x80

	if _, err := conn.Write(header); err != nil {
		log.Printf("Error writing WebSocket header to tunnel: %v", err)
		return err
	}
	if _, err := conn.Write(payload); err != nil {
		log.Printf("Error writing WebSocket payload to tunnel: %v", err)
		return err
	}
	log.Printf("Successfully wrote WebSocket message to tunnel. Type: %d, Payload length: %d", messageType, len(payload))
	return nil
}

func readWebSocketMessage(conn net.Conn) (int, []byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		log.Printf("Error setting read deadline: %v", err)
		return 0, nil, err
	}
	defer conn.SetReadDeadline(time.Time{})

	header := make([]byte, 2)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, nil, nil
		}
		log.Printf("Error reading WebSocket header from tunnel: %v", err)
		return 0, nil, err
	}

	fin := header[0]&0x80 != 0
	opcode := int(header[0] & 0x0F)

	payloadLen := int(header[1] & 0x7F)
	if payloadLen == 126 {
		extendedLen := make([]byte, 2)
		if _, err := io.ReadFull(conn, extendedLen); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(extendedLen))
	} else if payloadLen == 127 {
		extendedLen := make([]byte, 8)
		if _, err := io.ReadFull(conn, extendedLen); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(extendedLen))
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}

	if !fin {
		for {
			nextOpcode, nextPayload, err := readWebSocketMessage(conn)
			if err != nil {
				return 0, nil, err
			}
			if nextOpcode != WebSocketContinuationFrame {
				return 0, nil, fmt.Errorf("expected continuation frame")
			}
			payload = append(payload, nextPayload...)
			if nextOpcode&0x80 != 0 {
				break
			}
		}
	}

	return opcode, payload, nil
}