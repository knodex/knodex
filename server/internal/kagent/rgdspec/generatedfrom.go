// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package rgdspec deterministically verifies/backfills the
// knodex.io/generated-from traceability annotation on generated
// ResourceGraphDefinition specs (Story 50.2 AC #1). The RGD Builder agent's
// system prompt instructs per-resource annotation, but LLM compliance is
// probabilistic — this package is the server-side guarantee, applied to the
// A2A response text BEFORE it is persisted to the ResultStore.
//
// Pure functions only: no imports of other kagent packages, fully
// unit-testable in isolation. Every failure path is fail-soft — a backfill
// bug must never turn a successful agent run into a failed one or mangle the
// response.
package rgdspec

import (
	"errors"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// GeneratedFromAnnotation links a generated resource template to the user
// requirement that produced it.
const GeneratedFromAnnotation = "knodex.io/generated-from"

// maxRequirementRunes caps the backfilled annotation value. The full user
// message (≤8192 chars) is the backfill source — the run's InputSummary is
// truncated to 256 and must not be used — but an 8KiB annotation value helps
// nobody.
const maxRequirementRunes = 512

// rgdKind is the only kind this package mutates.
const rgdKind = "ResourceGraphDefinition"

// Fence patterns mirror the web extraction contract EXACTLY
// (web/src/components/agents/spec-extract.ts): first fenced ```yaml (or
// ```yml) block, falling back to the first bare ``` block.
var (
	yamlFenceRe = regexp.MustCompile("```(?:yaml|yml)[ \t]*\r?\n([\\s\\S]*?)```")
	bareFenceRe = regexp.MustCompile("```[ \t]*\r?\n([\\s\\S]*?)```")
)

// ExtractYAMLBlock finds the first fenced ```yaml (or ```yml) code block in
// text, falling back to the first bare ``` block — the web contract. It
// returns the raw INNER block content and its byte offsets [start, end) in
// text so a modified block can be spliced back between the original fences.
// ok is false when no fenced block exists or the block is blank.
func ExtractYAMLBlock(text string) (block string, start, end int, ok bool) {
	if text == "" {
		return "", 0, 0, false
	}
	for _, re := range []*regexp.Regexp{yamlFenceRe, bareFenceRe} {
		loc := re.FindStringSubmatchIndex(text)
		if loc == nil {
			continue
		}
		// loc[2]:loc[3] is the first capture group — the inner block content.
		inner := text[loc[2]:loc[3]]
		if strings.TrimSpace(inner) == "" {
			return "", 0, 0, false
		}
		return inner, loc[2], loc[3], true
	}
	return "", 0, 0, false
}

// EnsureGeneratedFrom walks spec.resources[].template of the RGD in block and
// adds GeneratedFromAnnotation (value: requirement, truncated to
// maxRequirementRunes) to each template's metadata.annotations when absent.
// An agent-provided value is never overwritten — the per-resource fragment
// the agent recorded is strictly more granular than the whole-prompt
// backfill.
//
// The yaml.v3 Node API is used (NOT a map round-trip) so key order and
// comments survive: the user sees this YAML verbatim in the preview/editor.
//
// Bail-unchanged (changed=false, err=nil) cases: the document is not a
// mapping, kind is not ResourceGraphDefinition, there is no spec.resources
// sequence, or the block holds more than one YAML document (multi-doc —
// leave untouched, fail-soft). Entries without a template mapping (e.g.
// externalRef-only entries) are skipped — nothing is generated for them.
func EnsureGeneratedFrom(block, requirement string) (out string, changed bool, err error) {
	dec := yaml.NewDecoder(strings.NewReader(block))

	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return block, false, err
	}

	// Multi-document blocks are left untouched: a second decode that does NOT
	// hit EOF means another document (or trailing garbage) follows.
	var second yaml.Node
	if err := dec.Decode(&second); !errors.Is(err, io.EOF) {
		return block, false, nil
	}

	root := documentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return block, false, nil
	}
	if kind := mapValue(root, "kind"); kind == nil || kind.Value != rgdKind {
		return block, false, nil
	}
	spec := mapValue(root, "spec")
	if spec == nil || spec.Kind != yaml.MappingNode {
		return block, false, nil
	}
	resources := mapValue(spec, "resources")
	if resources == nil || resources.Kind != yaml.SequenceNode {
		return block, false, nil
	}

	value := truncateRunes(requirement, maxRequirementRunes)
	for _, entry := range resources.Content {
		if entry.Kind != yaml.MappingNode {
			continue
		}
		template := mapValue(entry, "template")
		if template == nil || template.Kind != yaml.MappingNode {
			// externalRef-only (or malformed) entry — nothing generated here.
			continue
		}
		metadata := ensureMapping(template, "metadata")
		annotations := ensureMapping(metadata, "annotations")
		if annotations == nil {
			// metadata/annotations holds a non-mapping value (malformed agent
			// output) — leave the entry untouched rather than mangle it.
			continue
		}
		if mapValue(annotations, GeneratedFromAnnotation) != nil {
			continue // the agent already recorded a (more granular) value
		}
		annotations.Content = append(annotations.Content,
			scalarNode(GeneratedFromAnnotation),
			scalarNode(value),
		)
		changed = true
	}

	if !changed {
		return block, false, nil
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // k8s convention; yaml.v3 defaults to 4
	if err := enc.Encode(&doc); err != nil {
		return block, false, err
	}
	if err := enc.Close(); err != nil {
		return block, false, err
	}
	return buf.String(), true, nil
}

// BackfillResponse is the completion-path orchestrator: extract the fenced
// YAML block from the agent's response text, ensure the traceability
// annotation on every resource template, and splice the modified block back
// between the original fences. ANY error or no-op returns text unchanged —
// this function must NEVER fail the run.
func BackfillResponse(text, requirement string) string {
	block, start, end, ok := ExtractYAMLBlock(text)
	if !ok {
		return text
	}
	out, changed, err := EnsureGeneratedFrom(block, requirement)
	if err != nil || !changed {
		return text
	}
	return text[:start] + out + text[end:]
}

// documentRoot unwraps a DocumentNode to its single content node.
func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		return doc.Content[0]
	}
	return doc
}

// mapValue returns the value node for key in a mapping node, or nil.
// Mapping node Content alternates key/value pairs.
func mapValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// ensureMapping returns the mapping node at key in parent, creating an empty
// one when the key is missing. A `key:` with a null value (e.g. bare
// `metadata:`) is promoted in place to a mapping. nil is returned when parent
// is not a mapping or the existing value is some other non-mapping kind —
// the caller skips rather than mangling malformed agent output.
func ensureMapping(parent *yaml.Node, key string) *yaml.Node {
	if parent == nil || parent.Kind != yaml.MappingNode {
		return nil
	}
	if existing := mapValue(parent, key); existing != nil {
		if existing.Kind == yaml.MappingNode {
			return existing
		}
		if existing.Kind == yaml.ScalarNode && existing.Tag == "!!null" {
			// `metadata:` with no value — promote in place to a mapping.
			existing.Kind = yaml.MappingNode
			existing.Tag = "!!map"
			existing.Value = ""
			return existing
		}
		return nil
	}
	created := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, scalarNode(key), created)
	return created
}

// scalarNode builds a string scalar node.
func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// truncateRunes cuts s to at most n runes without splitting a UTF-8 sequence.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
