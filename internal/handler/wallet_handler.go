package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/shariski/simple-wallet/internal/model"
	"github.com/shariski/simple-wallet/internal/service"
)

type WalletHandler struct {
	Service  *service.WalletService
	Validate *validator.Validate
}

func NewWalletHandler(s *service.WalletService, v *validator.Validate) *WalletHandler {
	return &WalletHandler{
		Service:  s,
		Validate: v,
	}
}

func (h *WalletHandler) TopUp(w http.ResponseWriter, r *http.Request) {
	paramID := r.PathValue("id")
	walletID, err := uuid.Parse(paramID)
	if err != nil {
		http.Error(w, "invalid wallet id", http.StatusBadRequest)
		return
	}

	req := new(model.TopupWalletRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.ID = walletID

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	if err := h.Service.TopUp(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
