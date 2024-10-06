package server

import (
	"fmt"
	"log"
	"net/http"
)

type Server struct {
	httpPort string
}

func NewServer(httpPort string) *Server {
	return &Server{
		httpPort: httpPort,
	}
}

func (s *Server) StartServer() error {
	// Routes
	http.HandleFunc("/register_tunnel", s.handleRegisterTunnel)
	http.HandleFunc("/close_tunnel", s.handleCloseTunnel)

	log.Println("starting server on port", s.httpPort)

	addr := fmt.Sprintf(":%s", s.httpPort)
	err := http.ListenAndServe(addr, nil)

	if err != nil {
		return err
	}
	return nil
}
