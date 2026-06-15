// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package kagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a2aSuccessBody is a canonical message/send success response.
func a2aSuccessBody(contextID, text string) string {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"result": map[string]any{
			"contextId": contextID,
			"artifacts": []any{
				map[string]any{
					"parts": []any{map[string]any{"kind": "text", "text": text}},
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestA2AClient_Invoke_Success(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(a2aSuccessBody("ctx-123", "scale to 3 replicas")))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "alpha-apps", "helper", "what should I do?", "dev@example.com", "")
	require.NoError(t, err)

	// URL path: /api/a2a/{namespace}/{name}/ with trailing slash.
	assert.Equal(t, "/api/a2a/alpha-apps/helper/", gotPath)

	// JSON-RPC 2.0 envelope.
	assert.Equal(t, "2.0", gotBody["jsonrpc"])
	assert.Equal(t, "message/send", gotBody["method"])
	assert.NotEmpty(t, gotBody["id"])

	params, ok := gotBody["params"].(map[string]any)
	require.True(t, ok)
	message, ok := params["message"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", message["role"])
	assert.NotEmpty(t, message["messageId"])
	parts, ok := message["parts"].([]any)
	require.True(t, ok)
	require.Len(t, parts, 1)
	part, ok := parts[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "text", part["kind"])
	assert.Equal(t, "what should I do?", part["text"])

	metadata, ok := params["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "dev@example.com", metadata["knodex_actor"])

	// Parsed result: contextId + artifact text.
	assert.Equal(t, "ctx-123", result.ContextID)
	assert.Equal(t, "scale to 3 replicas", result.Text)
}

func TestA2AClient_Invoke_InputRequired_QuestionsFromStatusMessage(t *testing.T) {
	t.Parallel()
	// The RGD Builder enters input-required state after confirming resources.
	// The agent's clarifying questions are in status.message, NOT artifacts.
	// Both the resource-confirmation text (artifact) and the questions
	// (status.message) must be joined and InputRequired must be true.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-ir",
				"status": map[string]any{
					"state": "input-required",
					"message": map[string]any{
						"role": "agent",
						"parts": []any{
							map[string]any{"kind": "text", "text": "Please provide the following details:"},
							map[string]any{"kind": "data", "data": map[string]any{
								"type": "questions",
								"questions": []any{
									map[string]any{"question": "What name/prefix?", "options": []any{}},
									map[string]any{"question": "Which Azure region?", "options": []any{"eastus", "westus2"}},
								},
							}},
						},
					},
				},
				"artifacts": []any{
					map[string]any{
						"parts": []any{
							map[string]any{"kind": "text", "text": "Both resources are confirmed in the cluster."},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "ctx-ir", result.ContextID)
	assert.Equal(t,
		"Both resources are confirmed in the cluster.\n\nPlease provide the following details:",
		result.Text,
		"artifact text + status.message text must both appear")
	assert.True(t, result.InputRequired, "InputRequired must be set when status.state is input-required")
	require.Len(t, result.DataParts, 1, "data part from status.message must be captured")
	assert.Contains(t, string(result.DataParts[0]), "questions")
}

func TestA2AClient_Invoke_InputRequired_NoArtifacts(t *testing.T) {
	t.Parallel()
	// input-required with no artifacts — questions text comes solely from
	// status.message (no artifact text to prepend).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-ir2",
				"status": map[string]any{
					"state": "input-required",
					"message": map[string]any{
						"role":  "agent",
						"parts": []any{map[string]any{"kind": "text", "text": "What is your preferred region?"}},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "What is your preferred region?", result.Text)
	assert.True(t, result.InputRequired)
}

func TestA2AClient_Invoke_Completed_InputRequiredFalse(t *testing.T) {
	t.Parallel()
	// completed state must NOT set InputRequired.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-ok",
				"status":    map[string]any{"state": "completed"},
				"artifacts": []any{
					map[string]any{"parts": []any{map[string]any{"kind": "text", "text": "spec generated"}}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "spec generated", result.Text)
	assert.False(t, result.InputRequired)
}

func TestA2AClient_Invoke_JoinsMultipleTextParts(t *testing.T) {
	t.Parallel()
	// kagent splits a reply into multiple text parts when prose is interrupted
	// by a tool-call part in the event stream. Every text part must survive —
	// returning only the first truncates the response (the RGD Builder symptom:
	// reply cut off at "...details from you:" before the list it emitted as a
	// second part).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-multi",
				"artifacts": []any{
					map[string]any{
						"parts": []any{
							map[string]any{"kind": "text", "text": "Both resources are available. I need a few details from you:"},
							map[string]any{"kind": "data", "data": map[string]any{"ignored": true}},
							map[string]any{"kind": "text", "text": "- Resource group name\n- Location"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "ctx-multi", result.ContextID)
	assert.Equal(t,
		"Both resources are available. I need a few details from you:\n\n- Resource group name\n- Location",
		result.Text,
		"every text part must be joined, never just the first")
}

func TestA2AClient_Invoke_JoinsTextPartsAcrossArtifacts(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-arts",
				"artifacts": []any{
					map[string]any{"parts": []any{map[string]any{"kind": "text", "text": "intro"}}},
					map[string]any{"parts": []any{map[string]any{"kind": "text", "text": "outro"}}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "intro\n\noutro", result.Text)
}

func TestA2AClient_Invoke_JoinsHistoryFallbackTextParts(t *testing.T) {
	t.Parallel()
	// No artifacts ⇒ history fallback. The chosen (last agent) message may
	// itself carry multiple text parts — all of them must be joined.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-hmulti",
				"history": []any{
					map[string]any{"role": "user", "parts": []any{map[string]any{"kind": "text", "text": "question"}}},
					map[string]any{"role": "agent", "parts": []any{
						map[string]any{"kind": "text", "text": "part one"},
						map[string]any{"kind": "text", "text": "part two"},
					}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "part one\n\npart two", result.Text)
}

func TestA2AClient_Invoke_CapturesDataPartsAndKeepsText(t *testing.T) {
	t.Parallel()
	// A reply mixing text with a structured data part: the text renders, the
	// data part is captured verbatim (never dropped), and nothing counts as
	// unhandled.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-data",
				"artifacts": []any{
					map[string]any{
						"parts": []any{
							map[string]any{"kind": "text", "text": "Here is the spec."},
							map[string]any{"kind": "data", "data": map[string]any{"kind": "ResourceGraphDefinition", "ok": true}},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "Here is the spec.", result.Text)
	require.Len(t, result.DataParts, 1, "structured data part must be captured, not dropped")
	assert.JSONEq(t, `{"kind":"ResourceGraphDefinition","ok":true}`, string(result.DataParts[0]))
	assert.Equal(t, 0, result.UnhandledParts)
}

func TestA2AClient_Invoke_CountsUnhandledFileParts(t *testing.T) {
	t.Parallel()
	// File parts (and any unknown future kind) are neither rendered nor
	// captured as data — they must be COUNTED so the caller can surface them
	// rather than letting them silently vanish.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-file",
				"artifacts": []any{
					map[string]any{
						"parts": []any{
							map[string]any{"kind": "text", "text": "see attached"},
							map[string]any{"kind": "file", "file": map[string]any{"name": "spec.yaml"}},
							map[string]any{"kind": "mystery-future-kind"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "see attached", result.Text)
	assert.Empty(t, result.DataParts)
	assert.Equal(t, 2, result.UnhandledParts, "file part + unknown kind must be counted")
}

func TestA2AClient_Invoke_HistoryFallbackWhenNoArtifacts(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"result": map[string]any{
				"contextId": "ctx-h",
				"history": []any{
					map[string]any{"role": "user", "parts": []any{map[string]any{"kind": "text", "text": "question"}}},
					map[string]any{"role": "agent", "parts": []any{map[string]any{"kind": "text", "text": "first answer"}}},
					map[string]any{"role": "agent", "parts": []any{map[string]any{"kind": "text", "text": "final answer"}}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "ctx-h", result.ContextID)
	assert.Equal(t, "final answer", result.Text, "must take the LAST agent message, never the user's")
}

func TestA2AClient_Invoke_EmptyTextTolerated(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"contextId":"ctx-e"}}`))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	result, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "ctx-e", result.ContextID)
	assert.Equal(t, "", result.Text)
}

func TestA2AClient_Invoke_JSONRPCError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32600,"message":"agent not ready"}}`))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent not ready")
	assert.Contains(t, err.Error(), "-32600")
}

func TestA2AClient_Invoke_Non200(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestA2AClient_Invoke_MissingResult(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x"}`))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing result")
}

func TestA2AClient_Invoke_TransportError(t *testing.T) {
	t.Parallel()
	// Closed server ⇒ connection refused.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.Error(t, err)

	// NFR-A4/A7 (Story 50.1 review): transport errors flow into user-visible
	// run summaries and conversation results — the kagent endpoint URL must
	// be redacted, mapped to a stable actionable message instead.
	assert.NotContains(t, err.Error(), server.URL,
		"transport error must not leak the kagent endpoint URL")
	assert.NotContains(t, err.Error(), "/api/a2a/",
		"transport error must not leak the A2A path")
	assert.Contains(t, err.Error(), "kagent controller unreachable")
}

func TestA2AClient_Invoke_TimeoutRedacted(t *testing.T) {
	t.Parallel()
	// Server stalls past the request deadline ⇒ timeout. The handler is
	// released via `done` (closed before server.Close in LIFO defer order) —
	// blocking solely on r.Context() can deadlock Close when the server
	// never notices the client gave up.
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(done)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(ctx, "ns", "agent", "q", "a@b.c", "")
	require.Error(t, err)

	// NFR-A4/A7: the timeout message is stable, actionable and URL-free.
	assert.NotContains(t, err.Error(), server.URL,
		"timeout error must not leak the kagent endpoint URL")
	assert.Contains(t, err.Error(), "timed out waiting for the agent")
}

func TestA2AClient_Invoke_ContextCanceled(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(ctx, "ns", "agent", "q", "a@b.c", "")
	require.Error(t, err)
}

func TestA2AClient_Invoke_SendsContextIDWhenNonEmpty(t *testing.T) {
	t.Parallel()
	// When contextID is non-empty it must appear in params.configuration.contextId
	// so kagent resumes the same session.
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(a2aSuccessBody("ctx-resume", "ok")))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "ctx-prior")
	require.NoError(t, err)

	params, ok := gotBody["params"].(map[string]any)
	require.True(t, ok)
	cfg, ok := params["configuration"].(map[string]any)
	require.True(t, ok, "configuration must be present when contextID is non-empty")
	assert.Equal(t, "ctx-prior", cfg["contextId"])
}

func TestA2AClient_Invoke_OmitsConfigurationWhenContextIDEmpty(t *testing.T) {
	t.Parallel()
	// Empty contextID must NOT send configuration at all (clean first-turn request).
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(a2aSuccessBody("ctx-new", "ok")))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL)
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)

	params, ok := gotBody["params"].(map[string]any)
	require.True(t, ok)
	_, hasConfig := params["configuration"]
	assert.False(t, hasConfig, "configuration must be absent for empty contextID")
}

func TestNewA2AClient_TrimsTrailingSlash(t *testing.T) {
	t.Parallel()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(a2aSuccessBody("c", "t")))
	}))
	defer server.Close()

	client := NewA2AClient(server.URL + "/")
	_, err := client.Invoke(context.Background(), "ns", "agent", "q", "a@b.c", "")
	require.NoError(t, err)
	assert.Equal(t, "/api/a2a/ns/agent/", gotPath, "no double slash from a trailing-slash base URL")
}
