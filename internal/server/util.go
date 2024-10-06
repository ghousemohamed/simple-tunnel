package server

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

func (s *Server) sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func GenerateRandomSubdomain(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.New(rand.NewSource(time.Now().UnixNano()))
	subdomain := make([]byte, length)
	for i := range subdomain {
		subdomain[i] = charset[rand.Intn(len(charset))]
	}
	return string(subdomain)
}