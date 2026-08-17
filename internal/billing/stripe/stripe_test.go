package stripeadapter

import (
	"fmt"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

func TestVerifierAcceptsSupportedSignedSnapshotAndRejectsTamperingAndLiveMode(t *testing.T) {
	const secret = "whsec_credential_neutral_test_only"
	now := time.Now().UTC().Truncate(time.Second)
	payload := []byte(fmt.Sprintf(`{"id":"evt_test_invoice_paid","object":"event","api_version":%q,"created":%d,"livemode":false,"type":"invoice.paid","data":{"object":{"id":"in_test_invoice_paid","customer":"cus_test_customer","subscription":"sub_test_subscription","currency":"eur","amount_paid":2500,"metadata":{"alzette_payment_requirement_id":"pay_test_requirement"},"status":"paid"}}}`, stripe.APIVersion, now.Unix()))
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: now})
	verifier, err := NewVerifier(VerifierConfig{WebhookSecret: secret})
	if err != nil {
		t.Fatal(err)
	}
	event, err := verifier.Verify(payload, signed.Header, now)
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != "evt_test_invoice_paid" || event.Type != "invoice.paid" || event.PaymentRequirementID != "pay_test_requirement" || event.CustomerRef != "cus_test_customer" || event.InvoiceRef != "in_test_invoice_paid" || event.AmountMinor == nil || *event.AmountMinor != 2500 || event.Currency != "EUR" {
		t.Fatalf("verified event=%#v", event)
	}
	if _, err := verifier.Verify(append(payload, ' '), signed.Header, now); err == nil {
		t.Fatal("tampered signed payload verified")
	}
	livePayload := []byte(fmt.Sprintf(`{"id":"evt_test_live_event","object":"event","api_version":%q,"created":%d,"livemode":true,"type":"invoice.paid","data":{"object":{"id":"in_test_live_invoice","customer":"cus_test_customer","currency":"eur","amount_paid":2500,"metadata":{"alzette_payment_requirement_id":"pay_test_requirement"}}}}`, stripe.APIVersion, now.Unix()))
	liveSigned := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: livePayload, Secret: secret, Timestamp: now})
	if _, err := verifier.Verify(livePayload, liveSigned.Header, now); err == nil {
		t.Fatal("live-mode event passed the sandbox-default verifier")
	}
}
