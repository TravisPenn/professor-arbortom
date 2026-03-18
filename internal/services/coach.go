// Package services contains clients for external services.
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultSystemPrompt is used when COACH_SYSTEM_PROMPT is not set.
// COACH_SYSTEM_PROMPT env var fully replaces this when set.
const defaultSystemPrompt = `You are Professor Arbortom, a Nuzlocke coach for
Generation 3 Pokémon games (FireRed, LeafGreen, Ruby, Sapphire, Emerald).

Nuzlocke rules always in effect:
- Only the first Pokémon encountered in each new area may be caught.
- Any Pokémon that faints is permanently lost — treat it as unavailable.
- Additional rules may be active; they appear in the game data if set.

You will receive structured game data (party members, learnable moves, available
items, encounter options, upcoming opponents). Use it as ground truth. Fill gaps
from your general Pokémon knowledge, but note when you do so.

When answering, cover one or more of these categories where relevant:

  MOVE + EVOLUTION: Compare what the current form and its evolution(s) learn,
  and at what levels. Advise whether to evolve now or wait to learn a move first.

  CATCHES: Identify the first (or best) area to find a Pokémon that fills a
  type coverage gap relevant to the next gym. Mention the encounter level range.

  ITEMS: Reference available shop items, NPC gifts, and held-item strategy.
  Note prerequisite conditions for NPC gifts.

  TEAM THEME: Notice dominant types and suggest a Pokémon that would improve
  coverage or counter the next opponent.

Be concise. Use 3-5 sentences for simple questions; a short bullet list for
multi-part comparisons. Never suggest a fainted Pokémon.`

// CoachClient is a client for the Ollama /api/chat endpoint.
// All methods degrade gracefully — callers should check IsAvailable() before
// calling QueryCoach, but QueryCoach will also return a safe response on failure.
// Empty host = disabled.
type CoachClient struct {
	host         string // base URL, e.g. "http://ollama-lxc:11434"; empty = disabled
	model        string // Ollama model name, e.g. "qwen2.5:3b"
	systemPrompt string // optional; omitted from the request when empty
	http         *http.Client
}

// CoachPayload is the body sent to the AI Coach.
type CoachPayload struct {
	Candidates  CoachCandidates `json:"candidates"`
	Question    string          `json:"question"`
	ContextNote string          `json:"context_note,omitempty"`
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
func (c *CoachClient) QueryCoach(_ int, payload CoachPayload) CoachResponse {
	if c.host == "" {
		return CoachResponse{Available: false}
	}

	messages := make([]map[string]string, 0, 2)
	if c.systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": c.systemPrompt,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": formatPayload(payload),
	})

	reqBody, err := json.Marshal(map[string]interface{}{
		"model":      c.model,
		"stream":     false,
		"keep_alive": 0,
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

	return CoachResponse{
		Available: true,
		Answer:    result.Message.Content,
		Model:     result.Model,
		Truncated: result.DoneReason == "length",
	}
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
