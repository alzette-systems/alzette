package billing

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"alzette/internal/api"
	"alzette/internal/ids"
	"alzette/internal/platform"
)

const DefaultMaximumWebhookBody = 256 << 10

type Event struct {
	Provider             string
	ID                   string
	Type                 string
	ObjectRef            string
	PaymentRequirementID string
	CustomerRef          string
	SubscriptionRef      string
	InvoiceRef           string
	ObjectStatus         string
	PaymentStatus        string
	AmountMinor          *int64
	Currency             string
	PeriodStart          *time.Time
	PeriodEnd            *time.Time
	CancelAtPeriodEnd    *bool
	ProviderCreatedAt    time.Time
	PayloadDigest        [32]byte
	SignatureVerifiedAt  time.Time
}

type Verifier interface {
	Configured() bool
	Verify(payload []byte, signature string, now time.Time) (Event, error)
}

type WebhookStore interface {
	ReceiveBillingEvent(context.Context, Event) (duplicate bool, processed bool, err error)
	ApplyBillingEvent(context.Context, string, string, time.Time) error
}

type WebhookConfig struct {
	Store       WebhookStore
	Verifier    Verifier
	Clock       func() time.Time
	NewID       func(string) (string, error)
	MaximumBody int64
}

type WebhookHandler struct {
	store       WebhookStore
	verifier    Verifier
	clock       func() time.Time
	newID       func(string) (string, error)
	maximumBody int64
}

func NewWebhookHandler(config WebhookConfig) (*WebhookHandler, error) {
	if config.Store == nil || config.Verifier == nil {
		return nil, errors.New("billing webhook store and verifier are required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.NewID == nil {
		config.NewID = ids.New
	}
	if config.MaximumBody == 0 {
		config.MaximumBody = DefaultMaximumWebhookBody
	}
	if config.MaximumBody < 1024 || config.MaximumBody > 1<<20 {
		return nil, errors.New("billing webhook body limit is outside supported bounds")
	}
	return &WebhookHandler{store: config.Store, verifier: config.Verifier, clock: config.Clock, newID: config.NewID, maximumBody: config.MaximumBody}, nil
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := h.newID("whk")
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "internal_error", "api_error", "webhook request could not be initialised", "")
		return
	}
	w.Header().Set("X-Alzette-Request-ID", requestID)
	w.Header().Set("X-Request-ID", requestID)
	if r.URL.Path != "/webhooks/stripe" {
		api.WriteError(w, http.StatusNotFound, "not_found", "invalid_request_error", "resource not found", requestID)
		return
	}
	if r.Method != http.MethodPost {
		api.MethodNotAllowed(w, http.MethodPost, requestID)
		return
	}
	if !h.verifier.Configured() {
		api.WriteError(w, http.StatusServiceUnavailable, "payment_not_configured", "api_error", "billing webhook verification is not configured", requestID)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		api.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "invalid_request_error", "Content-Type must be application/json", requestID)
		return
	}
	signatures := r.Header.Values("Stripe-Signature")
	if len(signatures) != 1 || strings.TrimSpace(signatures[0]) == "" || strings.ContainsAny(signatures[0], "\r\n") {
		api.WriteError(w, http.StatusBadRequest, "invalid_signature", "authentication_error", "webhook signature verification failed", requestID)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maximumBody)
	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		api.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_webhook", "invalid_request_error", "webhook body is invalid", requestID)
		return
	}
	now := h.clock().UTC()
	event, err := h.verifier.Verify(payload, signatures[0], now)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "invalid_signature", "authentication_error", "webhook signature verification failed", requestID)
		return
	}
	event.PayloadDigest = sha256.Sum256(payload)
	event.SignatureVerifiedAt = now
	duplicate, processed, err := h.store.ReceiveBillingEvent(r.Context(), event)
	if err != nil {
		if errors.Is(err, platform.ErrInvalid) {
			api.WriteError(w, http.StatusBadRequest, "invalid_webhook", "invalid_request_error", "webhook event is invalid", requestID)
			return
		}
		api.WriteError(w, http.StatusServiceUnavailable, "webhook_ledger_unavailable", "api_error", "webhook receipt could not be recorded", requestID)
		return
	}
	if !processed {
		if err := h.store.ApplyBillingEvent(r.Context(), event.Provider, event.ID, now); err != nil {
			api.WriteError(w, http.StatusServiceUnavailable, "billing_reconciliation_pending", "api_error", "billing event is retained for reconciliation", requestID)
			return
		}
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{"received": true, "duplicate": duplicate})
}
