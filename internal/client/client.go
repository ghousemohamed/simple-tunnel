package client

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

type Client struct {
	httpPort string
	serverAddr string
	subdomain string
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

	// Add headers for WebSocket upgrade
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

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

	go func() {
		reader := bufio.NewReader(conn)
		for {
			req, err := http.ReadRequest(reader)
			if err != nil {
				if err == io.EOF {
					log.Println("Tunnel closed by server")
					return
				}
				log.Printf("Error reading request: %v", err)
				continue
			}

			if req.Method == "PING" {
				conn.Write([]byte("PONG\n"))
				continue
			}

			localResp, err := c.handleLocalRequest(req)
			if err != nil {
				log.Printf("Error handling local request: %v", err)
				continue
			}

			err = localResp.Write(conn)
			if err != nil {
				log.Printf("Error writing response to tunnel: %v", err)
			}
		}
	}()

	for {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			if err == io.EOF {
				log.Println("Tunnel closed by server")
				return nil
			}
			log.Printf("Error reading request: %v", err)
			continue
		}

		handleHTTP(c, conn, req)
	}
}

func (c *Client) handleLocalRequest(req *http.Request) (*http.Response, error) {
	localURL := fmt.Sprintf("http://localhost:%s%s", c.httpPort, req.URL.Path)
	if req.URL.RawQuery != "" {
		localURL += "?" + req.URL.RawQuery
	}

	localReq, err := http.NewRequest(req.Method, localURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("error creating local request: %v", err)
	}

	localReq.Header = req.Header

	return http.DefaultClient.Do(localReq)
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
