package faketarget

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeterministicTargetTimesOutBeforeOutputThenReturnsCompatibleResponse(t *testing.T) {
	const secret = "fake-provider-secret-canary"
	const prompt = "prompt-canary-that-must-not-be-reflected"
	handler, err := New(Config{Secret: secret, ProviderModel: DefaultProviderModel, TimeoutFirst: 1})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	requestBody := `{"model":"` + DefaultProviderModel + `","messages":[{"role":"user","content":"` + prompt + `"}],"stream":false}`
	call := func(ctx context.Context) (*http.Response, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(requestBody))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+secret)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Alzette-Request-ID", "req_test")
		return http.DefaultClient.Do(request)
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if response, err := call(timeoutContext); err == nil {
		response.Body.Close()
		t.Fatal("first deterministic call emitted a response instead of timing out")
	}

	response, err := call(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), ExpectedOutput) || !strings.Contains(string(body), DefaultProviderModel) {
		t.Fatalf("compatible response mismatch: status=%d length=%d", response.StatusCode, len(body))
	}
	if strings.Contains(string(body), secret) || strings.Contains(string(body), prompt) {
		t.Fatal("deterministic target reflected a credential or prompt canary")
	}
}

func TestDeterministicTargetFailsClosed(t *testing.T) {
	handler, err := New(Config{Secret: "expected-secret", ProviderModel: DefaultProviderModel})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, path, authorization, body string
		status                          int
	}{
		{"wrong path", "/other", "Bearer expected-secret", `{}`, http.StatusNotFound},
		{"missing auth", "/v1/chat/completions", "", `{}`, http.StatusUnauthorized},
		{"wrong model", "/v1/chat/completions", "Bearer expected-secret", `{"model":"other","messages":[{"role":"user","content":"x"}]}`, http.StatusBadRequest},
		{"unknown field", "/v1/chat/completions", "Bearer expected-secret", `{"model":"deterministic/slice0-v1","messages":[{"role":"user","content":"x"}],"target_url":"http://other"}`, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Alzette-Request-ID", "req_test")
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d length=%d", response.Code, response.Body.Len())
			}
		})
	}
}
