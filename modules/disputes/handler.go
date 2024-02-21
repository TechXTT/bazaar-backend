package disputes

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/gorilla/mux"
	"github.com/samber/do"
)

func NewDisputesHandler(i *do.Injector) (Handler, error) {
	return &disputesHandler{
		svc: do.MustInvoke[Service](i),
	}, nil
}

func (d *disputesHandler) CreateDispute(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	dispute := &Disputes{}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := r.MultipartForm.File["images"]

	log.Println(files)

	dispute.Dispute = r.FormValue("dispute")
	dispute.OrderID = uuid.FromStringOrNil(r.FormValue("orderId"))

	id, err := d.svc.CreateDispute(userId, dispute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, file := range files {
		filepath := "disputes/" + id + "/" + file.Filename
		imageURL, err := d.svc.SaveFile(file, filepath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		disputeImage := DisputeImages{
			DisputeID: uuid.FromStringOrNil(id),
			Image:     imageURL,
		}

		if err := d.svc.CreateDisputeImage(userId, &disputeImage); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (d *disputesHandler) GetDispute(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	vars := mux.Vars(r)
	id := vars["id"]

	dispute, err := d.svc.GetDispute(userId, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dispute)
}

func (d *disputesHandler) CloseDispute(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("user_id")
	vars := mux.Vars(r)
	id := vars["id"]

	if err := d.svc.CloseDispute(userId, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
