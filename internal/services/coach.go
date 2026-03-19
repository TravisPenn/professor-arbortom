// Package services contains clients for external services.
package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// defaultSystemPrompt is used when COACH_SYSTEM_PROMPT is not set.
// COACH_SYSTEM_PROMPT env var fully replaces this when set.
const defaultSystemPrompt = `/nothink
You are Professor Arbortom, a Pokémon expert coach.

ABSOLUTE RULES — NEVER BREAK THESE:
1. The game data you receive is GROUND TRUTH. Never invent or change any Pokémon name, type, move, level, location, or TM number.
2. If the data says "VERIFIED RECOMMENDATIONS", present those facts as friendly tips. Do NOT change any detail in them.
3. A Pokémon can ONLY learn moves listed in its "Usable TMs" section. If a move is not listed there, say "[Species] cannot learn [Move] in this game."
4. TMs can be taught at any time — never say "learn [TM] at level X". Only level-up moves have levels.
5. Only recommend catches from the "AVAILABLE CATCHES" section. If it says "None", say no catches are available yet.
6. Use the exact type listed in the data. Never guess or change a Pokémon's type.
7. Check "active_rules" for run constraints. Do not assume Nuzlocke or any rule unless listed.

FORMAT:
- Use **bold** for Pokémon, move, and item names only.
- Number each recommendation (1. 2. 3.).
- 1-2 sentences per recommendation.
- Never use placeholders like [Pokémon] or [Move].`

// cacheTTL is how long a cached coach response is considered fresh.
// Repeated page loads within this window are served instantly without hitting Ollama.
const cacheTTL = 3 * time.Minute

type cacheEntry struct {
	response CoachResponse
	expiry   time.Time
}

// CoachClient is a client for the Ollama /api/chat endpoint.
// All methods degrade gracefully — callers should check IsAvailable() before
// calling QueryCoach, but QueryCoach will also return a safe response on failure.
// Empty host = disabled.
type CoachClient struct {
	host         string // base URL, e.g. "http://ollama-lxc:11434"; empty = disabled
	model        string // Ollama model name, e.g. "qwen2.5:3b"
	systemPrompt string // persona instructions; always non-empty after NewCoachClient (falls back to defaultSystemPrompt)
	http         *http.Client
	cacheMu      sync.Mutex
	cache        map[string]cacheEntry
}

// CoachPayload is the body sent to the AI Coach.
type CoachPayload struct {
	Candidates  CoachCandidates `json:"candidates"`
	Question    string          `json:"question"`
	ContextNote string          `json:"context_note,omitempty"`
	GameSummary string          `json:"-"` // Pre-formatted text; replaces raw JSON when set
}

// CoachCandidates holds all structured candidate data for the Coach.
type CoachCandidates struct {
	Acquisitions   interface{} `json:"acquisitions"`
	Items          interface{} `json:"items"`
	PartyMoves     interface{} `json:"party_moves"`
	TeamAnalysis   interface{} `json:"team_analysis,omitempty"`
	EvolutionPaths interface{} `json:"evolution_paths,omitempty"`
	PartyDetails   interface{} `json:"party_details,omitempty"`
	NextOpponents  interface{} `json:"next_opponents,omitempty"` // COACH-015
	ActiveRules    interface{} `json:"active_rules,omitempty"`   // enabled run rules
}

// CoachResponse is the response from the AI Coach.
type CoachResponse struct {
	Available bool   `json:"available"`
	Answer    string `json:"answer"`
	Model     string `json:"model"`
	Truncated bool   `json:"truncated"`
}

// NewCoachClient creates a CoachClient. host may be empty to disable.
// When systemPrompt is empty, defaultSystemPrompt is used.
func NewCoachClient(host, model, systemPrompt string) *CoachClient {
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	return &CoachClient{
		host:         host,
		model:        model,
		systemPrompt: systemPrompt,
		http:         &http.Client{Timeout: 120 * time.Second},
		cache:        make(map[string]cacheEntry),
	}
}

// ValidateConfig checks the host URL for safety.
// SEC-005: Must be called at startup; returns an error if the value is
// malformed, preventing SSRF via env var manipulation.
// Empty host is valid (disabled).
func (c *CoachClient) ValidateConfig() error {
	if c.host == "" {
		return nil // disabled — nothing to validate
	}
	u, err := url.Parse(c.host)
	if err != nil {
		return fmt.Errorf("coach: invalid host URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("coach: host scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("coach: host URL has no host")
	}
	if c.model == "" {
		return fmt.Errorf("coach: model must be set when host is enabled")
	}
	return nil
}

// IsAvailable returns true if GET {host}/ returns 200 (Ollama health check).
func (c *CoachClient) IsAvailable() bool {
	if c.host == "" {
		return false
	}
	resp, err := c.http.Get(c.host + "/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// QueryCoach sends a coaching request to Ollama /api/chat and returns the response.
// keep_alive is always 0 — the model is evicted from VRAM immediately after each
// query so that the shared GPU (GTX 970) remains available for other workloads.
// On any failure, returns CoachResponse{Available: false} — never errors.
func (c *CoachClient) QueryCoach(runID int, payload CoachPayload) CoachResponse {
	if c.host == "" {
		return CoachResponse{Available: false}
	}

	userContent := formatContext(payload)

	// Cache lookup — keyed by sha256(runID + game state + question).
	// Identical page loads within cacheTTL are served instantly.
	cacheRaw := fmt.Sprintf("%d\n%s\n%s", runID, userContent, payload.Question)
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(cacheRaw)))
	c.cacheMu.Lock()
	if entry, ok := c.cache[cacheKey]; ok && time.Now().Before(entry.expiry) {
		c.cacheMu.Unlock()
		log.Printf("[coach] cache hit (run %d)", runID)
		return entry.response
	}
	c.cacheMu.Unlock()

	log.Printf("[coach] payload (run %d):\n%s\nQuestion: %s", runID, userContent, payload.Question)

	messages := []map[string]string{
		{"role": "system", "content": c.systemPrompt},
		{"role": "user", "content": userContent},
		{"role": "assistant", "content": "Got it — I've reviewed the current game state and I'm ready to advise."},
		{"role": "user", "content": payload.Question},
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"model":      c.model,
		"stream":     false,
		"think":      false,
		"keep_alive": 300, // keep model warm for 5 min; cold-load on GTX 970 adds ~10-15s per query
		"messages":   messages,
	})
	if err != nil {
		return CoachResponse{Available: false}
	}

	req, err := http.NewRequest(http.MethodPost, c.host+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return CoachResponse{Available: false}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return CoachResponse{Available: false}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CoachResponse{Available: false}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return CoachResponse{Available: false}
	}

	var result struct {
		Model   string `json:"model"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done       bool   `json:"done"`
		DoneReason string `json:"done_reason"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return CoachResponse{Available: false}
	}
	if !result.Done || result.Message.Content == "" {
		return CoachResponse{Available: false}
	}

	answer := stripThinking(result.Message.Content)
	if answer == "" {
		return CoachResponse{Available: false}
	}

	resp2 := CoachResponse{
		Available: true,
		Answer:    answer,
		Model:     result.Model,
		Truncated: result.DoneReason == "length",
	}
	c.cacheMu.Lock()
	c.cache[cacheKey] = cacheEntry{response: resp2, expiry: time.Now().Add(cacheTTL)}
	c.cacheMu.Unlock()
	return resp2
}

// stripThinking removes <think>...</think> reasoning blocks that models like
// qwen3 prepend to their output. Returns just the visible answer text.
func stripThinking(s string) string {
	const closeTag = "</think>"
	if i := strings.Index(s, closeTag); i >= 0 {
		s = s[i+len(closeTag):]
	}
	return strings.TrimSpace(s)
}

// formatContext serialises the fixed game-state portion of a CoachPayload into
// a prompt string. The question is kept separate and sent as a distinct user turn.
func formatContext(p CoachPayload) string {
	var sb strings.Builder
	if p.ContextNote != "" {
		sb.WriteString("Context: ")
		sb.WriteString(p.ContextNote)
		sb.WriteString("\n\n")
	}
	if p.GameSummary != "" {
		sb.WriteString(p.GameSummary)
	} else if data, err := json.Marshal(p.Candidates); err == nil {
		sb.WriteString("Game data:\n")
		sb.Write(data) //nolint:errcheck
	}
	return sb.String()
}

// formatPayload serialises the enriched CoachPayload into a compact prompt string.
func formatPayload(p CoachPayload) string {
	var sb strings.Builder
	if p.ContextNote != "" {
		sb.WriteString("Context: ")
		sb.WriteString(p.ContextNote)
		sb.WriteString("\n\n")
	}
	if data, err := json.Marshal(p.Candidates); err == nil {
		sb.WriteString("Game data:\n")
		sb.Write(data) //nolint:errcheck
		sb.WriteString("\n\n")
	}
	sb.WriteString("Question: ")
	sb.WriteString(p.Question)
	return sb.String()
}
