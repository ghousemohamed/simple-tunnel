package server

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	// "strings"
	"sync"
	"time"
	"github.com/gorilla/websocket"
)

type TunnelConnection struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

type TunnelServer struct {
	tunnel     map[string]*TunnelConnection
	tunnelsLock sync.RWMutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

const (
	WebSocketContinuationFrame = 0
	WebSocketTextFrame         = 1
	WebSocketBinaryFrame       = 2
	WebSocketCloseFrame        = 8
	WebSocketPingFrame         = 9
	WebSocketPongFrame         = 10
)

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{
		tunnel: make(map[string]*TunnelConnection),
	}
}

func (ts *TunnelServer) handleTunnelRequest(w http.ResponseWriter, r *http.Request) {
	// subdomain := strings.Split(r.Host, ".")[0]

	ts.tunnelsLock.RLock()
	tunnel, ok := ts.tunnel["hello"]
	ts.tunnelsLock.RUnlock()

	if !ok {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	if tunnel == nil {
		http.Error(w, "No available tunnels", http.StatusServiceUnavailable)
		return
	}

	if websocket.IsWebSocketUpgrade(r) {
		ts.handleWebSocketUpgrade(w, r, tunnel)
		return
	}

	for {
		log.Printf("Waiting for request from tunnel...")
		req, err := http.ReadRequest(bufio.NewReader(tunnel.conn))
		if err != nil {
			if err == io.EOF {
				log.Println("Tunnel closed by client")
				return
			}
			log.Printf("Error reading request from tunnel: %v", err)
			continue
		}

		log.Printf("Received request from tunnel: %s %s", req.Method, req.URL.Path)

		if err := req.Write(tunnel.writer); err != nil {
			log.Printf("Error forwarding request: %v", err)
			http.Error(w, "Error forwarding request", http.StatusInternalServerError)
			return
		}

		if err := tunnel.writer.Flush(); err != nil {
			log.Printf("Error flushing request: %v", err)
			http.Error(w, "Error flushing request", http.StatusInternalServerError)
			return
		}

		resp, err := http.ReadResponse(tunnel.reader, r)
		if err != nil {
			http.Error(w, "Error reading response from tunnel", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func (ts *TunnelServer) handleTunnelOpen(w http.ResponseWriter, r *http.Request) {
	subdomain := r.URL.Query().Get("subdomain")
	if subdomain == "" {
		http.Error(w, "Subdomain not specified", http.StatusBadRequest)
		return
	}

	conn, bufrw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("Hijack error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tunnelConn := &TunnelConnection{
		conn:   conn,
		reader: bufrw.Reader,
		writer: bufrw.Writer,
	}

	ts.tunnelsLock.Lock()
	ts.tunnel[subdomain] = tunnelConn
	ts.tunnelsLock.Unlock()

	log.Printf("Tunnel opened for subdomain: %s", subdomain)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"\r\n"

	_, err = conn.Write([]byte(response))
	if err != nil {
		log.Printf("Error writing response: %v", err)
		ts.removeTunnel(subdomain, conn)
		return
	}

	// Start a goroutine to keep the connection alive and monitor for closure
	go ts.monitorConnection(subdomain, tunnelConn)
}

func (ts *TunnelServer) monitorConnection(subdomain string, tunnelConn *TunnelConnection) {
	defer ts.removeTunnel(subdomain, tunnelConn.conn)

	// Keep-alive loop
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// For now let's not do anything here
		default:
			// Check if the connection is closed
			one := []byte{}
			tunnelConn.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, err := tunnelConn.conn.Read(one)
			tunnelConn.conn.SetReadDeadline(time.Time{}) // Reset the read deadline
			if err == io.EOF {
				log.Printf("Client closed the connection for subdomain: %s", subdomain)
				return
			} else if err != nil && !err.(net.Error).Timeout() {
				log.Printf("Error reading from connection: %v", err)
				return
			}
		}
		time.Sleep(1 * time.Second) // Small delay to prevent tight loop
	}
}

func (ts *TunnelServer) removeTunnel(subdomain string, conn net.Conn) {
	ts.tunnelsLock.Lock()
	defer ts.tunnelsLock.Unlock()
	delete(ts.tunnel, subdomain)

	conn.Close()
}

func (ts *TunnelServer) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request, tunnel *TunnelConnection) {
	serverConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade server connection: %v", err)
		return
	}
	defer serverConn.Close()

	upgradeReq := &http.Request{
		Method: http.MethodGet,
		URL:    r.URL,
		Header: make(http.Header),
	}
	for k, v := range r.Header {
		upgradeReq.Header[k] = v
	}
	upgradeReq.Header.Set("Connection", "Upgrade")
	upgradeReq.Header.Set("Upgrade", "websocket")

	if err := upgradeReq.Write(tunnel.writer); err != nil {
		log.Printf("Failed to send upgrade request to client: %v", err)
		return
	}
	if err := tunnel.writer.Flush(); err != nil {
		log.Printf("Failed to flush upgrade request to client: %v", err)
		return
	}

	upgradeResp, err := http.ReadResponse(tunnel.reader, upgradeReq)
	if err != nil {
		log.Printf("Failed to read upgrade response from client: %v", err)
		return
	}
	if upgradeResp.StatusCode != http.StatusSwitchingProtocols {
		log.Printf("Client failed to upgrade connection: %v", upgradeResp.Status)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			messageType, p, err := serverConn.ReadMessage()
			if err != nil {
				log.Printf("Error reading from server WebSocket: %v", err)
				return
			}
			log.Printf("Server received message: %s", string(p))
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
			if err := writeWebSocketMessage(tunnel.conn, wsMessageType, p); err != nil {
				log.Printf("Error writing to tunnel: %v", err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			messageType, p, err := readWebSocketMessage(tunnel.conn)
			if err != nil {
				log.Printf("Error reading from tunnel: %v", err)
				return
			}
			if messageType == 0 && p == nil {
				continue
			}
			log.Printf("Server received message from tunnel: %s", string(p))
			var wsMessageType int
			switch messageType {
			case WebSocketTextFrame:
				wsMessageType = websocket.TextMessage
			case WebSocketBinaryFrame:
				wsMessageType = websocket.BinaryMessage
			case WebSocketCloseFrame:
				wsMessageType = websocket.CloseMessage
			case WebSocketPingFrame:
				wsMessageType = websocket.PingMessage
			case WebSocketPongFrame:
				wsMessageType = websocket.PongMessage
			default:
				log.Printf("Unknown message type: %d", messageType)
				continue
			}
			if err := serverConn.WriteMessage(wsMessageType, p); err != nil {
				log.Printf("Error writing to server WebSocket: %v", err)
				return
			}
		}
	}()

	wg.Wait()
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
		return err
	}
	if _, err := conn.Write(payload); err != nil {
		return err
	}
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
			log.Printf("Error reading extended payload length (16-bit): %v", err)
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(extendedLen))
	} else if payloadLen == 127 {
		extendedLen := make([]byte, 8)
		if _, err := io.ReadFull(conn, extendedLen); err != nil {
			log.Printf("Error reading extended payload length (64-bit): %v", err)
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(extendedLen))
	}

	log.Printf("Reading WebSocket message. Opcode: %d, Payload length: %d", opcode, payloadLen)

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		log.Printf("Error reading WebSocket payload from tunnel: %v", err)
		return 0, nil, err
	}

	log.Printf("Successfully read WebSocket message from tunnel. Opcode: %d, Payload: %s", opcode, string(payload))

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