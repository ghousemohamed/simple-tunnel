package server

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{
		tunnel: make(map[string]*TunnelConnection),
	}
}

func (ts *TunnelServer) handleTunnelRequest(w http.ResponseWriter, r *http.Request) {
	subdomain := strings.Split(r.Host, ".")[0]

	ts.tunnelsLock.RLock()
	tunnel, ok := ts.tunnel[subdomain]
	ts.tunnelsLock.RUnlock()

	if !ok {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	if tunnel == nil {
		http.Error(w, "No available tunnels", http.StatusServiceUnavailable)
		return
	}

	if err := r.Write(tunnel.writer); err != nil {
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