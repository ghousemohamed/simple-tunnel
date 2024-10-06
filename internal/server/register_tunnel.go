package server

import "net/http"


func (s *Server) handleRegisterTunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Tunnel registered",
	}
	s.sendJSONResponse(w, response)
}