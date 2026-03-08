// Package services contains clients for external services.
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// ZeroClaw is a client for the ZeroClaw LLM gateway (LXC 130).
// All methods degrade gracefully — callers should check IsAvailable() before
// calling QueryCoach, but QueryCoach will also return a safe response on failure.
type ZeroClaw struct {
	gateway string // base URL, e.g. "http://your-llm-host:42617"; empty = disabled
	agent   string // agent profile name in ZeroClaw config
	http    *http.Client
}

// CoachPayload is the body sent to ZeroClaw.
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
}

// CoachResponse is the response from ZeroClaw.
type CoachResponse struct {
	Available bool   `json:"available"`
	Answer    string `json:"answer"`
	Model     string `json:"model"`
	Truncated bool   `json:"truncated"`
}

// validAgent matches alphanumeric names with hyphens and underscores.
var validAgent = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// NewZeroClaw creates a ZeroClaw client. gateway may be empty to disable.
func NewZeroClaw(gateway, agent string) *ZeroClaw {
	return &ZeroClaw{
		gateway: gateway,
		agent:   agent,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// ValidateConfig checks the gateway URL and agent name for safety.
// SEC-005: Must be called at startup; returns an error if the values are
// malformed, preventing SSRF via env var manipulation or path traversal
// via the agent name.
func (z *ZeroClaw) ValidateConfig() error {
	if z.gateway == "" {
		return nil // disabled — nothing to validate
	}
	u, err := url.Parse(z.gateway)
	if err != nil {
		return fmt.Errorf("zeroclaw: invalid gateway URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("zeroclaw: gateway scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("zeroclaw: gateway URL has no host")
	}
	if z.agent == "" {
		return fmt.Errorf("zeroclaw: agent name is required when gateway is set")
	}
	if !validAgent.MatchString(z.agent) {
		return fmt.Errorf("zeroclaw: agent name %q contains invalid characters (allowed: a-z A-Z 0-9 - _)", z.agent)
	}
	return nil
}

// IsAvailable returns true if the ZEROCLAW_GATEWAY env var is set and the
// service is reachable. Cheap check — does not call the AI endpoint.
func (z *ZeroClaw) IsAvailable() bool {
	if z.gateway == "" {
		return false
	}
	resp, err := z.http.Get(z.gateway + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// QueryCoach sends a coaching request to ZeroClaw and returns the response.
// On any failure, returns CoachResponse{Available: false} — never errors.
func (z *ZeroClaw) QueryCoach(runID int, payload CoachPayload) CoachResponse {
	if z.gateway == "" {
		return CoachResponse{Available: false}
	}

	body, err := json.Marshal(map[string]interface{}{
		"agent":   z.agent,
		"run_id":  runID,
		"payload": payload,
	})
	if err != nil {
		return CoachResponse{Available: false}
	}

	url := fmt.Sprintf("%s/agent/%s/query", z.gateway, z.agent)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return CoachResponse{Available: false}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := z.http.Do(req)
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
		Answer    string `json:"answer"`
		Model     string `json:"model"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return CoachResponse{Available: false}
	}

	return CoachResponse{
		Available: true,
		Answer:    result.Answer,
		Model:     result.Model,
		Truncated: result.Truncated,
	}
}
