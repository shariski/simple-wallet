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

func (h *WalletHandler) Create(w http.ResponseWriter, r *http.Request) {
	req := new(model.CreateWalletRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	res, err := h.Service.Create(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (h *WalletHandler) TopUp(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "idempotency key not found in request", http.StatusBadRequest)
		return
	}

	paramID := r.PathValue("id")
	walletID, err := uuid.Parse(paramID)
	if err != nil {
		http.Error(w, "invalid wallet id", http.StatusBadRequest)
		return
	}

	req := new(model.TopupWalletRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.IdempotencyKey = idempotencyKey
	req.ID = walletID

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	wallet, err := h.Service.TopUp(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(wallet)
}

func (h *WalletHandler) Payment(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "idempotency key not found in request", http.StatusBadRequest)
		return
	}

	paramID := r.PathValue("id")
	walletID, err := uuid.Parse(paramID)
	if err != nil {
		http.Error(w, "invalid wallet id", http.StatusBadRequest)
		return
	}

	req := new(model.PaymentWalletRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.IdempotencyKey = idempotencyKey
	req.ID = walletID

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	res, err := h.Service.Payment(r.Context(), req)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (h *WalletHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		http.Error(w, "idempotency key not found in request", http.StatusBadRequest)
		return
	}

	req := new(model.TransferWalletRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.IdempotencyKey = idempotencyKey

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	sender, receiver, err := h.Service.Transfer(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res := []*model.WalletResponse{sender, receiver}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (h *WalletHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	paramID := r.PathValue("id")
	walletID, err := uuid.Parse(paramID)
	if err != nil {
		http.Error(w, "invalid wallet id", http.StatusBadRequest)
		return
	}

	req := new(model.SuspendWalletRequest)
	req.ID = walletID

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	res, err := h.Service.Suspend(r.Context(), req)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (h *WalletHandler) Status(w http.ResponseWriter, r *http.Request) {
	paramID := r.PathValue("id")
	walletID, err := uuid.Parse(paramID)
	if err != nil {
		http.Error(w, "invalid wallet id", http.StatusBadRequest)
		return
	}

	req := new(model.StatusWalletRequest)
	req.ID = walletID

	if err := h.Validate.Struct(req); err != nil {
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	res, err := h.Service.Status(r.Context(), req)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}
