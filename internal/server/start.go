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
	server := http.Server{
		Addr: fmt.Sprintf(":%s", s.httpPort),
		Handler: http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		}),
	}

	log.Println("starting server on port", s.httpPort)

	err := server.ListenAndServe()

	if err != nil {
		return err
	}
	return nil
}