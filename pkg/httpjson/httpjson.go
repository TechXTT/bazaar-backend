package httpjson

import (
	"encoding/json"
	"log"
	"net/http"
)

func WriteError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// WriteAppError routes a handler error through a single chokepoint (BE-21).
// For 5xx it logs the real error server-side and returns a generic message so
// internal/DB detail never reaches the client; for 4xx the error text (our
// validation/sentinel messages, which are caller-safe) is returned as-is.
func WriteAppError(w http.ResponseWriter, code int, err error) {
	if code >= http.StatusInternalServerError {
		if err != nil {
			log.Printf("request failed with %d: %v", code, err)
		}
		WriteError(w, code, "internal server error")
		return
	}
	msg := "bad request"
	if err != nil {
		msg = err.Error()
	}
	WriteError(w, code, msg)
}

func WriteJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}
