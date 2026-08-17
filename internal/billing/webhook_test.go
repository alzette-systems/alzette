package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type verifierStub struct {
	event Event
	err   error
}

func (v verifierStub) Configured() bool { return true }
func (v verifierStub) Verify([]byte, string, time.Time) (Event, error) {
	return v.event, v.err
}

type webhookStoreStub struct {
	received Event
	applied  int
}

func (s *webhookStoreStub) ReceiveBillingEvent(_ context.Context, event Event) (bool, bool, error) {
	s.received = event
	return false, false, nil
}
func (s *webhookStoreStub) ApplyBillingEvent(context.Context, string, string, time.Time) error {
	s.applied++
	return nil
}

func TestWebhookHandlerBoundedRawVerificationAndSafeResponse(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	store := &webhookStoreStub{}
	handler, err := NewWebhookHandler(WebhookConfig{Store: store, Verifier: verifierStub{event: Event{Provider: "stripe", ID: "evt_test_event", Type: "invoice.paid", ObjectRef: "in_test_invoice", ProviderCreatedAt: now}}, Clock: func() time.Time { return now }, NewID: func(string) (string, error) { return "whk_safe_request", nil }, MaximumBody: 1024})
	if err != nil {
		t.Fatal(err)
	}
	const rawCanary = "raw-webhook-content-canary"
	request := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(`{"object":"`+rawCanary+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Stripe-Signature", "t=1,v1=test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.applied != 1 || store.received.PayloadDigest == [32]byte{} || store.received.SignatureVerifiedAt != now {
		t.Fatalf("webhook status=%d applied=%d", response.Code, store.applied)
	}
	if strings.Contains(response.Body.String(), rawCanary) {
		t.Fatal("webhook response reflected raw content")
	}
	duplicateHeader := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(`{}`))
	duplicateHeader.Header.Set("Content-Type", "application/json")
	duplicateHeader.Header.Add("Stripe-Signature", "one")
	duplicateHeader.Header.Add("Stripe-Signature", "two")
	duplicateResponse := httptest.NewRecorder()
	handler.ServeHTTP(duplicateResponse, duplicateHeader)
	if duplicateResponse.Code != http.StatusBadRequest {
		t.Fatalf("duplicate signature status=%d", duplicateResponse.Code)
	}
	oversized := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(strings.Repeat("x", 2048)))
	oversized.Header.Set("Content-Type", "application/json")
	oversized.Header.Set("Stripe-Signature", "one")
	oversizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(oversizedResponse, oversized)
	if oversizedResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized webhook status=%d", oversizedResponse.Code)
	}
}
