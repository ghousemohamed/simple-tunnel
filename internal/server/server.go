package server

import (
	"fmt"
	"log"
	"net/http"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	ts := NewTunnelServer()
	// Routes
	http.HandleFunc("/", ts.handleTunnelRequest)
	http.HandleFunc("/_tunnel", ts.handleTunnelOpen)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", s.httpPort),
		Handler: nil,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
	return nil
}
