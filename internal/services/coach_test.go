package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestCoachClient(t *testing.T, serverURL, model string) *CoachClient {
	t.Helper()
	cc := NewCoachClient(serverURL, model, "")
	cc.http.Timeout = 2 * time.Second
	return cc
}

// ── IsAvailable ───────────────────────────────────────────────────────────────

func TestIsAvailable_NoHost(t *testing.T) {
	cc := NewCoachClient("", "qwen2.5:3b", "")
	if cc.IsAvailable() {
		t.Fatal("IsAvailable should be false when host is empty")
	}
}

func TestIsAvailable_ServiceUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	if !cc.IsAvailable() {
		t.Fatal("IsAvailable should be true when server returns 200 /")
	}
}

func TestIsAvailable_ServiceReturns500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	if cc.IsAvailable() {
		t.Fatal("IsAvailable should be false when server returns 500")
	}
}

// ── QueryCoach ────────────────────────────────────────────────────────────────

func TestQueryCoach_NoHost(t *testing.T) {
	cc := NewCoachClient("", "qwen2.5:3b", "")
	resp := cc.QueryCoach(1, CoachPayload{Question: "what pokemon should I use?"})
	if resp.Available {
		t.Fatal("QueryCoach should return Available=false with no host")
	}
}

func TestQueryCoach_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"model":       "qwen2.5:3b",
				"message":     map[string]string{"role": "assistant", "content": "Use Charizard!"},
				"done":        true,
				"done_reason": "stop",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	resp := cc.QueryCoach(42, CoachPayload{Question: "best pick?"})

	if !resp.Available {
		t.Fatal("expected Available=true on success")
	}
	if resp.Answer != "Use Charizard!" {
		t.Errorf("Answer = %q, want %q", resp.Answer, "Use Charizard!")
	}
	if resp.Model != "qwen2.5:3b" {
		t.Errorf("Model = %q, want %q", resp.Model, "qwen2.5:3b")
	}
	if resp.Truncated {
		t.Error("Truncated should be false when done_reason is stop")
	}
}

func TestQueryCoach_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	resp := cc.QueryCoach(1, CoachPayload{Question: "help!"})
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

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	resp := cc.QueryCoach(1, CoachPayload{Question: "help!"})
	if resp.Available {
		t.Fatal("QueryCoach should return Available=false on malformed JSON")
	}
}

func TestQueryCoach_KeepAliveIsZero(t *testing.T) {
	var reqBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			json.NewDecoder(r.Body).Decode(&reqBody) //nolint:errcheck
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"model":       "qwen2.5:3b",
				"message":     map[string]string{"role": "assistant", "content": "ok"},
				"done":        true,
				"done_reason": "stop",
			})
		}
	}))
	defer srv.Close()

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	cc.QueryCoach(1, CoachPayload{Question: "test"})

	ka, ok := reqBody["keep_alive"]
	if !ok {
		t.Fatal("request body must contain keep_alive field")
	}
	// JSON numbers unmarshal to float64 when target is interface{}
	if ka.(float64) != 0 {
		t.Errorf("keep_alive = %v, want 0", ka)
	}
}

func TestQueryCoach_Truncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"model":       "qwen2.5:3b",
			"message":     map[string]string{"role": "assistant", "content": "partial answer..."},
			"done":        true,
			"done_reason": "length",
		})
	}))
	defer srv.Close()

	cc := newTestCoachClient(t, srv.URL, "qwen2.5:3b")
	resp := cc.QueryCoach(1, CoachPayload{Question: "tell me everything"})

	if !resp.Available {
		t.Fatal("expected Available=true")
	}
	if !resp.Truncated {
		t.Error("expected Truncated=true when done_reason is length")
	}
}

func TestQueryCoach_NoSystemPromptWhenEmpty(t *testing.T) {
	var reqBody struct {
		Messages []map[string]string `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&reqBody) //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"model":       "qwen2.5:3b",
			"message":     map[string]string{"role": "assistant", "content": "ok"},
			"done":        true,
			"done_reason": "stop",
		})
	}))
	defer srv.Close()

	// Empty system prompt — messages should only contain the user message.
	cc := NewCoachClient(srv.URL, "qwen2.5:3b", "")
	cc.http.Timeout = 2 * time.Second
	cc.QueryCoach(1, CoachPayload{Question: "test"})

	if len(reqBody.Messages) != 1 {
		t.Errorf("expected 1 message (user only), got %d", len(reqBody.Messages))
	}
	if len(reqBody.Messages) > 0 && reqBody.Messages[0]["role"] != "user" {
		t.Errorf("first message role = %q, want %q", reqBody.Messages[0]["role"], "user")
	}
}

// ── ValidateConfig (SEC-005) ──────────────────────────────────────────────────

func TestValidateConfig_EmptyHost(t *testing.T) {
	cc := NewCoachClient("", "", "")
	if err := cc.ValidateConfig(); err != nil {
		t.Fatalf("empty host should be valid (disabled): %v", err)
	}
}

func TestValidateConfig_ValidHTTP(t *testing.T) {
	cc := NewCoachClient("http://ollama-lxc:11434", "qwen2.5:3b", "")
	if err := cc.ValidateConfig(); err != nil {
		t.Fatalf("valid http host should pass: %v", err)
	}
}

func TestValidateConfig_ValidHTTPS(t *testing.T) {
	cc := NewCoachClient("https://ollama.example.com", "qwen2.5:3b", "")
	if err := cc.ValidateConfig(); err != nil {
		t.Fatalf("valid https host should pass: %v", err)
	}
}

func TestValidateConfig_BadScheme(t *testing.T) {
	cc := NewCoachClient("ftp://evil.com", "qwen2.5:3b", "")
	if err := cc.ValidateConfig(); err == nil {
		t.Fatal("ftp scheme should be rejected")
	}
}

func TestValidateConfig_NoHost(t *testing.T) {
	cc := NewCoachClient("http://", "qwen2.5:3b", "")
	if err := cc.ValidateConfig(); err == nil {
		t.Fatal("empty host should be rejected")
	}
}
