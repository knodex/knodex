// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package kagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// a2aTimeout bounds a single agent invocation. Agent responses are
	// LLM-bound and routinely take tens of seconds — this client must NOT
	// reuse the presence checker's 3s health-check client (presence.go).
	a2aTimeout = 120 * time.Second

	// a2aMaxResponseBytes caps how much of an A2A response body is read,
	// protecting against a misbehaving controller streaming unbounded output.
	a2aMaxResponseBytes = 4 << 20 // 4 MiB
)

// A2AResult is the parsed outcome of a successful A2A message/send call.
type A2AResult struct {
	// ContextID is the kagent session id (response result.contextId) —
	// stored on the run record as kagentSessionId.
	ContextID string
	// Text is the full text content of the response — every text part joined
	// across artifacts AND, when the task is input-required, the agent's
	// clarifying-question message (falling back to the last agent message in
	// history when artifacts are empty). Empty is fine.
	Text string
	// DataParts holds structured (kind="data") parts captured verbatim, in
	// order — from artifacts AND, when input-required, from status.message.
	// Callers may parse these as needed; today they are surfaced in the run
	// result so the web can render structured question forms.
	DataParts []json.RawMessage
	// UnhandledParts counts response parts that were neither rendered as text
	// nor captured as data (file parts, or any future/unknown kind). A
	// non-zero value is the signal to extend part handling — the caller logs
	// it rather than dropping it on the floor.
	UnhandledParts int
	// InputRequired is true when the A2A task returned status "input-required":
	// the agent asked clarifying questions (in Text) and is waiting for the
	// user to provide answers before it can generate a final response.
	InputRequired bool
	// InputTokens, OutputTokens, TotalTokens are extracted from the kagent
	// usage metadata (result.metadata["kagent_usage_metadata"]).
	// Zero when the controller did not report usage.
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// A2AClient invokes kagent agents over the A2A JSON-RPC 2.0 endpoint:
// POST {base}/api/a2a/{namespace}/{agent-name}/ with method "message/send".
type A2AClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewA2AClient creates an A2A client for the given kagent controller base
// URL (e.g. http://kagent-controller.kagent.svc.cluster.local:8083).
func NewA2AClient(baseURL string) *A2AClient {
	return &A2AClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: a2aTimeout},
	}
}

// a2aPart is a single message part. Discriminated by Kind ("text"/"data"/
// "file" per the A2A spec). Text is set on request parts and text responses;
// Data/File carry the structured/file payloads on response parts and are
// captured verbatim (omitempty keeps request envelopes text-only).
type a2aPart struct {
	Kind string          `json:"kind"`
	Text string          `json:"text"`
	Data json.RawMessage `json:"data,omitempty"`
	File json.RawMessage `json:"file,omitempty"`
}

// a2aMessage is the params.message envelope of a message/send request.
type a2aMessage struct {
	Role      string    `json:"role"`
	Parts     []a2aPart `json:"parts"`
	MessageID string    `json:"messageId"`
}

// a2aRequest is the JSON-RPC 2.0 request envelope.
type a2aRequest struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      string    `json:"id"`
	Method  string    `json:"method"`
	Params  a2aParams `json:"params"`
}

type a2aParams struct {
	Message       a2aMessage        `json:"message"`
	Configuration *a2aConfiguration `json:"configuration,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// a2aConfiguration carries session-continuation settings for message/send.
type a2aConfiguration struct {
	// ContextID resumes an existing kagent session so the agent retains memory
	// of prior turns. Empty means start a fresh session.
	ContextID string `json:"contextId,omitempty"`
}

// Response parsing — defensively typed: every field is optional and absent
// fields simply yield zero values.
type a2aResponse struct {
	Result *a2aResponseResult `json:"result"`
	Error  *a2aError          `json:"error"`
}

type a2aError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type a2aResponseResult struct {
	ContextID string `json:"contextId"`
	// Status holds the task state. When state is "input-required" the agent
	// paused and its clarifying-question message is in Status.Message.
	Status    *a2aTaskStatus `json:"status,omitempty"`
	Artifacts []a2aArtifact  `json:"artifacts"`
	History   []a2aHistory   `json:"history"`
	// Metadata is kagent's task-level metadata map. The key
	// "kagent_usage_metadata" carries LLM token counts (promptTokenCount,
	// candidatesTokenCount, totalTokenCount).
	Metadata map[string]any `json:"metadata,omitempty"`
}

// a2aTaskStatus is the status field of an A2A Task (message/send response).
type a2aTaskStatus struct {
	State string `json:"state"`
	// Message is set when state is "input-required" and holds the agent's
	// clarifying questions as message parts.
	Message *a2aMessage `json:"message,omitempty"`
}

type a2aArtifact struct {
	Parts []a2aPart `json:"parts"`
}

type a2aHistory struct {
	Role  string    `json:"role"`
	Parts []a2aPart `json:"parts"`
}

// Invoke sends a user message to the agent at {namespace}/{name} and waits
// for the synchronous JSON-RPC response. actor is the authenticated Knodex
// user identity — carried in request metadata for kagent-side observability;
// the authoritative actor record lives on the Knodex run record.
// contextID, when non-empty, resumes an existing kagent session (A2A
// configuration.contextId) so the agent retains memory of prior turns.
func (c *A2AClient) Invoke(ctx context.Context, namespace, name, message, actor, contextID string) (*A2AResult, error) {
	params := a2aParams{
		Message: a2aMessage{
			Role:      "user",
			Parts:     []a2aPart{{Kind: "text", Text: message}},
			MessageID: uuid.NewString(),
		},
		Metadata: map[string]any{"knodex_actor": actor},
	}
	if contextID != "" {
		params.Configuration = &a2aConfiguration{ContextID: contextID}
	}
	reqBody := a2aRequest{
		JSONRPC: "2.0",
		ID:      uuid.NewString(),
		Method:  "message/send",
		Params:  params,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal a2a request: %w", err)
	}

	// Trailing slash is required by the kagent A2A router (epic-verified).
	endpoint := fmt.Sprintf("%s/api/a2a/%s/%s/", c.baseURL, url.PathEscape(namespace), url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build a2a request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// NFR-A4/A7: never propagate the raw transport error — url.Error
		// embeds the full kagent endpoint URL (and DNS failures embed the
		// host), and this text flows into user-visible run summaries and
		// conversation results. Map to stable, actionable messages instead.
		var uerr *url.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &uerr) && uerr.Timeout()) {
			return nil, fmt.Errorf("a2a call failed: timed out waiting for the agent to respond")
		}
		return nil, fmt.Errorf("a2a call failed: kagent controller unreachable")
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, a2aMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read a2a response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("a2a endpoint returned status %d", resp.StatusCode)
	}

	var parsed a2aResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse a2a response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("a2a error %d: %s", parsed.Error.Code, parsed.Error.Message)
	}
	if parsed.Result == nil {
		return nil, fmt.Errorf("a2a response missing result")
	}

	text, data, unhandled := extractParts(parsed.Result)
	inputRequired := parsed.Result.Status != nil && parsed.Result.Status.State == "input-required"
	inputTokens, outputTokens, totalTokens := extractTokenCounts(parsed.Result)
	return &A2AResult{
		ContextID:      parsed.Result.ContextID,
		Text:           strings.Join(text, "\n\n"),
		DataParts:      data,
		UnhandledParts: unhandled,
		InputRequired:  inputRequired,
		InputTokens:    inputTokens,
		OutputTokens:   outputTokens,
		TotalTokens:    totalTokens,
	}, nil
}

// extractTokenCounts reads promptTokenCount/candidatesTokenCount/totalTokenCount
// from result.metadata["kagent_usage_metadata"]. Returns zeros when absent.
func extractTokenCounts(result *a2aResponseResult) (input, output, total int) {
	if result.Metadata == nil {
		return
	}
	usage, ok := result.Metadata["kagent_usage_metadata"].(map[string]any)
	if !ok {
		return
	}
	if v, ok := usage["promptTokenCount"].(float64); ok {
		input = int(v)
	}
	if v, ok := usage["candidatesTokenCount"].(float64); ok {
		output = int(v)
	}
	if v, ok := usage["totalTokenCount"].(float64); ok {
		total = int(v)
	}
	return
}

// extractParts kind-discriminates the response into text segments (joined by
// the caller), captured data parts, and a count of parts left unhandled. It
// reads all artifact parts, in order, then appends the status.message parts
// when the task is input-required (the agent paused to ask clarifying
// questions — those questions live in status.message, not artifacts). When
// neither source yields text, it falls back to the last agent message in
// history. Empty text is a valid outcome.
//
// kagent (ADK-based) splits a single reply into multiple parts whenever the
// prose is interrupted by a non-text part in the underlying event stream
// (e.g. a tool call to check CRD presence between two prose segments).
// Taking only the FIRST text part silently truncated replies — the observed
// symptom was a response cut off mid-sentence, just before a list the agent
// emitted as a second part. Every text part is now joined; structured data
// parts are captured instead of dropped; anything else is counted so the
// caller can surface it.
func extractParts(result *a2aResponseResult) (text []string, data []json.RawMessage, unhandled int) {
	for _, artifact := range result.Artifacts {
		t, d, u := classifyParts(artifact.Parts)
		text = append(text, t...)
		data = append(data, d...)
		unhandled += u
	}
	// input-required: the agent's clarifying questions are in status.message,
	// not artifacts. Append them so the full turn (e.g. "resources confirmed
	// + questions") is returned. Return immediately — the history fallback is
	// irrelevant when the agent provided an explicit status message.
	if result.Status != nil && result.Status.State == "input-required" && result.Status.Message != nil {
		t, d, u := classifyParts(result.Status.Message.Parts)
		text = append(text, t...)
		data = append(data, d...)
		unhandled += u
		return text, data, unhandled
	}
	if len(text) > 0 {
		return text, data, unhandled
	}
	// No text in artifacts — fall back to the last agent message for text,
	// merging any data/unhandled parts it carries with what we already saw.
	for i := len(result.History) - 1; i >= 0; i-- {
		h := result.History[i]
		if h.Role == "user" {
			continue
		}
		t, d, u := classifyParts(h.Parts)
		if len(t) > 0 {
			return t, append(data, d...), unhandled + u
		}
	}
	return text, data, unhandled
}

// classifyParts sorts one flat part list by kind: text parts (non-empty) into
// text, structured data parts into data, everything else (file parts, unknown
// future kinds, or malformed data parts) into the unhandled count. An empty
// or absent kind with text is treated leniently as text — some emitters omit
// the discriminator. A genuinely empty part (no kind, no text) is ignored.
func classifyParts(parts []a2aPart) (text []string, data []json.RawMessage, unhandled int) {
	for _, p := range parts {
		switch p.Kind {
		case "", "text":
			if p.Text != "" {
				text = append(text, p.Text)
			}
		case "data":
			if len(p.Data) > 0 {
				data = append(data, p.Data)
			} else {
				unhandled++
			}
		default:
			unhandled++
		}
	}
	return text, data, unhandled
}
