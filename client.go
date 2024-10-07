package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

var (
	localPort  = flag.Int("local-port", 8000, "Local port to forward")
	subdomain  = flag.String("subdomain", "", "Desired subdomain")
	serverAddr = flag.String("server", "simpletunnel.me:80", "Address of the public server")
)

func main() {
	flag.Parse()

	tunnelURL := fmt.Sprintf("http://%s/_tunnel?subdomain=%s", *serverAddr, *subdomain)

	var conn net.Conn
	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	req, err := http.NewRequest("GET", tunnelURL, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	err = req.Write(conn)
	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to open tunnel: %s", resp.Status)
	}
	log.Printf("Your site is now available at: https://%s.%s", *subdomain, *serverAddr)

	for {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			if err == io.EOF {
				log.Println("Tunnel closed by server")
				return
			}
			log.Printf("Error reading request: %v", err)
			continue
		}

		handleHTTP(conn, req)
	}
}

func handleHTTP(conn net.Conn, req *http.Request) {
	localURL := fmt.Sprintf("http://localhost:%d%s", *localPort, req.URL.Path)
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