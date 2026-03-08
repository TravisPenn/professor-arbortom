package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestZeroClaw creates a ZeroClaw client pointing at the given server URL.
func newTestZeroClaw(t *testing.T, serverURL, agent string) *ZeroClaw {
	t.Helper()
	zc := NewZeroClaw(serverURL, agent)
	// Shorten timeout so tests fail fast
	zc.http.Timeout = 2 * time.Second
	return zc
}

// ── IsAvailable ───────────────────────────────────────────────────────────────

func TestIsAvailable_NoGateway(t *testing.T) {
	zc := NewZeroClaw("", "test-agent")
	if zc.IsAvailable() {
		t.Fatal("IsAvailable should be false when gateway is empty")
	}
}

func TestIsAvailable_ServiceUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	zc := newTestZeroClaw(t, srv.URL, "test-agent")
	if !zc.IsAvailable() {
		t.Fatal("IsAvailable should be true when server returns 200 /health")
	}
}

func TestIsAvailable_ServiceReturns500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	zc := newTestZeroClaw(t, srv.URL, "test-agent")
	if zc.IsAvailable() {
		t.Fatal("IsAvailable should be false when server returns 500")
	}
}

// ── QueryCoach ────────────────────────────────────────────────────────────────

func TestQueryCoach_NoGateway(t *testing.T) {
	zc := NewZeroClaw("", "test-agent")
	resp := zc.QueryCoach(1, CoachPayload{Question: "what pokemon should I use?"})
	if resp.Available {
		t.Fatal("QueryCoach should return Available=false with no gateway")
	}
}

func TestQueryCoach_Success(t *testing.T) {
	want := struct {
		Answer    string `json:"answer"`
		Model     string `json:"model"`
		Truncated bool   `json:"truncated"`
	}{
		Answer: "Use Charizard!",
		Model:  "gpt-4o",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agent/coach/query" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(want) //nolint:errcheck
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	zc := newTestZeroClaw(t, srv.URL, "coach")
	resp := zc.QueryCoach(42, CoachPayload{Question: "best pick?"})

	if !resp.Available {
		t.Fatal("expected Available=true on success")
	}
	if resp.Answer != want.Answer {
		t.Errorf("Answer = %q, want %q", resp.Answer, want.Answer)
	}
	if resp.Model != want.Model {
		t.Errorf("Model = %q, want %q", resp.Model, want.Model)
	}
}

func TestQueryCoach_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	zc := newTestZeroClaw(t, srv.URL, "coach")
	resp := zc.QueryCoach(1, CoachPayload{Question: "help!"})
	if resp.Available {
		t.Fatal("QueryCoach should return Available=false when server errors")
	}
}

func TestQueryCoach_MalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json")) //nolint:errcheck
	}))
	defer srv.Close()

	zc := newTestZeroClaw(t, srv.URL, "coach")
	resp := zc.QueryCoach(1, CoachPayload{Question: "help!"})
	if resp.Available {
		t.Fatal("QueryCoach should return Available=false on malformed JSON")
	}
}

// ── ValidateConfig (SEC-005) ──────────────────────────────────────────────────

func TestValidateConfig_EmptyGateway(t *testing.T) {
	zc := NewZeroClaw("", "")
	if err := zc.ValidateConfig(); err != nil {
		t.Fatalf("empty gateway should be valid (disabled): %v", err)
	}
}

func TestValidateConfig_ValidHTTP(t *testing.T) {
	zc := NewZeroClaw("http://llm-host:42617", "coach")
	if err := zc.ValidateConfig(); err != nil {
		t.Fatalf("valid http gateway should pass: %v", err)
	}
}

func TestValidateConfig_ValidHTTPS(t *testing.T) {
	zc := NewZeroClaw("https://llm.example.com", "my-agent_01")
	if err := zc.ValidateConfig(); err != nil {
		t.Fatalf("valid https gateway should pass: %v", err)
	}
}

func TestValidateConfig_BadScheme(t *testing.T) {
	zc := NewZeroClaw("ftp://evil.com", "agent")
	if err := zc.ValidateConfig(); err == nil {
		t.Fatal("ftp scheme should be rejected")
	}
}

func TestValidateConfig_NoHost(t *testing.T) {
	zc := NewZeroClaw("http://", "agent")
	if err := zc.ValidateConfig(); err == nil {
		t.Fatal("empty host should be rejected")
	}
}

func TestValidateConfig_AgentPathTraversal(t *testing.T) {
	zc := NewZeroClaw("http://host:1234", "../etc/passwd")
	if err := zc.ValidateConfig(); err == nil {
		t.Fatal("agent with path traversal should be rejected")
	}
}

func TestValidateConfig_AgentWithSlash(t *testing.T) {
	zc := NewZeroClaw("http://host:1234", "foo/bar")
	if err := zc.ValidateConfig(); err == nil {
		t.Fatal("agent with slash should be rejected")
	}
}

func TestValidateConfig_MissingAgent(t *testing.T) {
	zc := NewZeroClaw("http://host:1234", "")
	if err := zc.ValidateConfig(); err == nil {
		t.Fatal("empty agent with set gateway should be rejected")
	}
}
