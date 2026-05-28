package disputes

import (
	"encoding/json"
	"net/http"

	"github.com/TechXTT/bazaar-backend/pkg/httpjson"
	"github.com/gorilla/mux"
	"github.com/samber/do"
)

func NewDisputesHandler(i *do.Injector) (Handler, error) {
	return &disputesHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (d *disputesHandler) GetDisputes(w http.ResponseWriter, r *http.Request) {
	walletAddress := r.Header.Get("user_id")

	disputes, err := d.svc.ListDisputes(walletAddress)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(disputes)
}

func (d *disputesHandler) GetDispute(w http.ResponseWriter, r *http.Request) {
	walletAddress := r.Header.Get("user_id")
	vars := mux.Vars(r)
	orderId := vars["orderId"]

	dispute, err := d.svc.GetDispute(walletAddress, orderId)
	if err != nil {
		httpjson.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dispute)
}

func (d *disputesHandler) GetEvidence(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderId := vars["orderId"]

	evidence, err := d.svc.GetEvidence(orderId)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(evidence)
}
