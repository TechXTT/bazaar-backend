package httpjson

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteAppError covers BE-21: 5xx responses must not leak the underlying
// error to the client, while 4xx responses carry the (caller-safe) message.
func TestWriteAppError(t *testing.T) {
	secret := errors.New("pq: relation \"orders\" does not exist at 127.0.0.1:5432")

	t.Run("5xx hides the real error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteAppError(rec, http.StatusInternalServerError, secret)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status: got %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "orders") || strings.Contains(body, "5432") {
			t.Fatalf("5xx body leaked internal detail: %q", body)
		}
		var payload map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["error"] != "internal server error" {
			t.Fatalf("error message: got %q", payload["error"])
		}
	})

	t.Run("4xx passes the safe message", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteAppError(rec, http.StatusNotFound, errors.New("not found"))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status: got %d", rec.Code)
		}
		var payload map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload["error"] != "not found" {
			t.Fatalf("error message: got %q", payload["error"])
		}
	})
}
