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
	"sync/atomic"
)

type TunnelConnection struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	inUse  int32
}

type TunnelServer struct {
	tunnels     map[string][]*TunnelConnection
	tunnelsLock sync.RWMutex
}

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{
		tunnels: make(map[string][]*TunnelConnection),
	}
}

func NewTunnelConnection(conn net.Conn) *TunnelConnection {
	return &TunnelConnection{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		inUse:  0,
	}
}

func (ts *TunnelServer) handleTunnelRequest(w http.ResponseWriter, r *http.Request) {
	subdomain := strings.Split(r.Host, ".")[0]

	ts.tunnelsLock.RLock()
	tunnels, ok := ts.tunnels[subdomain]
	ts.tunnelsLock.RUnlock()

	if !ok || len(tunnels) == 0 {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	var tunnel *TunnelConnection
	for _, t := range tunnels {
		if atomic.CompareAndSwapInt32(&t.inUse, 0, 1) {
			tunnel = t
			break
		}
	}

	if tunnel == nil {
		http.Error(w, "No available tunnels", http.StatusServiceUnavailable)
		return
	}

	defer atomic.StoreInt32(&tunnel.inUse, 0)

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

func (ts *TunnelServer) handleHTTP(w http.ResponseWriter, r *http.Request, tunnel net.Conn) {
	tunnel.SetDeadline(time.Now().Add(30 * time.Second))
	defer tunnel.SetDeadline(time.Time{})

	if err := r.Write(tunnel); err != nil {
		http.Error(w, "Error forwarding request", http.StatusInternalServerError)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(tunnel), r)
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

	ts.tunnelsLock.Lock()
	ts.tunnels[subdomain] = append(ts.tunnels[subdomain], &TunnelConnection{
		conn:   conn,
		reader: bufrw.Reader,
		writer: bufrw.Writer,
		inUse:  0,
	})
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

	// Start a goroutine to keep the connection alive
	go ts.keepAlive(subdomain, conn)
}

func (ts *TunnelServer) keepAlive(subdomain string, conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := conn.Write([]byte("PING\n"))
			if err != nil {
				log.Printf("Error sending keep-alive to tunnel %s: %v", subdomain, err)
				ts.removeTunnel(subdomain, conn)
				return
			}
		}
	}
}

func (ts *TunnelServer) removeTunnel(subdomain string, conn net.Conn) {
	ts.tunnelsLock.Lock()
	defer ts.tunnelsLock.Unlock()

	tunnels := ts.tunnels[subdomain]
	for i, t := range tunnels {
		if t.conn == conn {
			ts.tunnels[subdomain] = append(tunnels[:i], tunnels[i+1:]...)
			break
		}
	}

	if len(ts.tunnels[subdomain]) == 0 {
		delete(ts.tunnels, subdomain)
	}

	conn.Close()
}