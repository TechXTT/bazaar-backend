package disputes

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/TechXTT/bazaar-backend/services/middleware"
	"github.com/gorilla/mux"
	"github.com/samber/do"
)

// statusForError maps service sentinel errors to HTTP status codes.
func statusForError(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func NewDisputesHandler(i *do.Injector) (Handler, error) {
	return &disputesHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (d *disputesHandler) GetDisputes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)

	disputes, err := d.svc.ListDisputes(userID)
	if err != nil {
		httpjson.WriteAppError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(disputes)
}

func (d *disputesHandler) GetDispute(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	vars := mux.Vars(r)
	orderId := vars["orderId"]
	if orderId == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "orderId is required")
		return
	}

	dispute, err := d.svc.GetDispute(userID, orderId)
	if err != nil {
		httpjson.WriteAppError(w, statusForError(err), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dispute)
}

func (d *disputesHandler) GetEvidence(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r)
	vars := mux.Vars(r)
	orderId := vars["orderId"]
	if orderId == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "orderId is required")
		return
	}

	evidence, err := d.svc.GetEvidence(userID, orderId)
	if err != nil {
		httpjson.WriteAppError(w, statusForError(err), err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(evidence)
}
