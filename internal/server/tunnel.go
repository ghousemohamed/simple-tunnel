package server

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

type TunnelServer struct {
	tunnels     map[string]net.Conn
	tunnelsLock sync.RWMutex
}

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{
		tunnels:  make(map[string]net.Conn),
	}
}

func (ts *TunnelServer) handleTunnelRequest(w http.ResponseWriter, r *http.Request) {
	subdomain := strings.Split(r.Host, ".")[0]

	ts.tunnelsLock.RLock()
	tunnel, ok := ts.tunnels[subdomain]
	ts.tunnelsLock.RUnlock()

	if !ok {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	ts.handleHTTP(w, r, tunnel)
}

func (ts *TunnelServer) handleHTTP(w http.ResponseWriter, r *http.Request, tunnel net.Conn) {
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

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ts.tunnelsLock.Lock()
	ts.tunnels[subdomain] = conn
	ts.tunnelsLock.Unlock()

	log.Printf("Tunnel opened for subdomain: %s", subdomain)
	response := "HTTP/1.1 200 OK\r\n" +
        "Content-Type: text/plain\r\n" +
        "Content-Length: 13\r\n" +
        "\r\n" +
        "Tunnel opened"

  conn.Write([]byte(response))
}