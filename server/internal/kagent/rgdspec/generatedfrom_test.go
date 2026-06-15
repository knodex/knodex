// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rgdspec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const annotatedRGD = `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-stack
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppStack
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          annotations:
            knodex.io/generated-from: "a web app"
`

const unannotatedRGD = `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-stack
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppStack
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
    - id: redis
      template:
        apiVersion: redis.example.io/v1
        kind: RedisCluster
`

func TestExtractYAMLBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		wantBlock string
		wantOK    bool
	}{
		{
			name:      "first fenced yaml block",
			text:      "Here is your spec:\n```yaml\nkind: Spec\n```\nDeploy it.",
			wantBlock: "kind: Spec\n",
			wantOK:    true,
		},
		{
			name:      "yml fence alias",
			text:      "```yml\nkind: Thing\n```",
			wantBlock: "kind: Thing\n",
			wantOK:    true,
		},
		{
			name:      "bare fence fallback",
			text:      "Some prose\n```\nkind: Thing\nname: x\n```\nmore prose",
			wantBlock: "kind: Thing\nname: x\n",
			wantOK:    true,
		},
		{
			name:      "yaml fence preferred over earlier bare fence",
			text:      "```\nnot the spec\n```\n\n```yaml\nkind: Spec\n```",
			wantBlock: "kind: Spec\n",
			wantOK:    true,
		},
		{
			name:   "no fenced block (the no-match path)",
			text:   "No matching CRDs found for: Redis",
			wantOK: false,
		},
		{name: "empty input", text: "", wantOK: false},
		{name: "empty block", text: "```yaml\n```", wantOK: false},
		{
			name:      "first of several yaml blocks",
			text:      "```yaml\nkind: First\n```\n```yaml\nkind: Second\n```",
			wantBlock: "kind: First\n",
			wantOK:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			block, start, end, ok := ExtractYAMLBlock(tt.text)
			require.Equal(t, tt.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, tt.wantBlock, block)
			// Offsets must address the returned inner content exactly so a
			// modified block can be spliced back between the fences.
			assert.Equal(t, tt.wantBlock, tt.text[start:end])
		})
	}
}

func TestEnsureGeneratedFrom_AlreadyAnnotated_Passthrough(t *testing.T) {
	t.Parallel()
	out, changed, err := EnsureGeneratedFrom(annotatedRGD, "whole prompt")
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, annotatedRGD, out, "fully annotated spec must pass through byte-identical")
}

func TestEnsureGeneratedFrom_BackfillsMissingAnnotations(t *testing.T) {
	t.Parallel()
	out, changed, err := EnsureGeneratedFrom(unannotatedRGD, "web app with redis")
	require.NoError(t, err)
	require.True(t, changed)

	parsed := parseRGD(t, out)
	resources := parsed["spec"].(map[string]interface{})["resources"].([]interface{})
	require.Len(t, resources, 2)
	for _, r := range resources {
		template := r.(map[string]interface{})["template"].(map[string]interface{})
		metadata := template["metadata"].(map[string]interface{})
		annotations := metadata["annotations"].(map[string]interface{})
		assert.Equal(t, "web app with redis", annotations[GeneratedFromAnnotation])
	}
}

func TestEnsureGeneratedFrom_PartiallyAnnotated_FillsOnlyMissing(t *testing.T) {
	t.Parallel()
	block := `kind: ResourceGraphDefinition
spec:
  resources:
    - id: annotated
      template:
        kind: Deployment
        metadata:
          annotations:
            knodex.io/generated-from: "the agent fragment"
    - id: bare
      template:
        kind: Service
`
	out, changed, err := EnsureGeneratedFrom(block, "whole prompt")
	require.NoError(t, err)
	require.True(t, changed)

	parsed := parseRGD(t, out)
	resources := parsed["spec"].(map[string]interface{})["resources"].([]interface{})
	first := annotationOf(t, resources[0])
	second := annotationOf(t, resources[1])
	assert.Equal(t, "the agent fragment", first, "agent-provided value must NEVER be overwritten")
	assert.Equal(t, "whole prompt", second)
}

func TestEnsureGeneratedFrom_CreatesMetadataAndAnnotations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		block string
	}{
		{
			name: "template without metadata",
			block: `kind: ResourceGraphDefinition
spec:
  resources:
    - id: a
      template:
        kind: Deployment
`,
		},
		{
			name: "metadata without annotations",
			block: `kind: ResourceGraphDefinition
spec:
  resources:
    - id: a
      template:
        kind: Deployment
        metadata:
          name: web
`,
		},
		{
			name: "null metadata promoted",
			block: `kind: ResourceGraphDefinition
spec:
  resources:
    - id: a
      template:
        kind: Deployment
        metadata:
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, changed, err := EnsureGeneratedFrom(tt.block, "req")
			require.NoError(t, err)
			require.True(t, changed)
			parsed := parseRGD(t, out)
			resources := parsed["spec"].(map[string]interface{})["resources"].([]interface{})
			assert.Equal(t, "req", annotationOf(t, resources[0]))
		})
	}
}

func TestEnsureGeneratedFrom_SkipsExternalRefEntries(t *testing.T) {
	t.Parallel()
	block := `kind: ResourceGraphDefinition
spec:
  resources:
    - id: existing-secret
      externalRef:
        apiVersion: v1
        kind: Secret
        metadata:
          name: creds
`
	out, changed, err := EnsureGeneratedFrom(block, "req")
	require.NoError(t, err)
	assert.False(t, changed, "externalRef-only entries are not generated — nothing to annotate")
	assert.Equal(t, block, out)
}

func TestEnsureGeneratedFrom_BailUnchangedCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		block string
	}{
		{name: "non-RGD kind", block: "kind: Deployment\nspec:\n  replicas: 1\n"},
		{name: "non-mapping document", block: "- a\n- b\n"},
		{name: "scalar document", block: "just a string\n"},
		{name: "no spec.resources", block: "kind: ResourceGraphDefinition\nspec:\n  schema:\n    kind: X\n"},
		{name: "resources not a sequence", block: "kind: ResourceGraphDefinition\nspec:\n  resources: oops\n"},
		{
			name:  "multi-document block",
			block: "kind: ResourceGraphDefinition\nspec:\n  resources:\n    - id: a\n      template:\n        kind: D\n---\nkind: Service\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, changed, err := EnsureGeneratedFrom(tt.block, "req")
			require.NoError(t, err)
			assert.False(t, changed)
			assert.Equal(t, tt.block, out)
		})
	}
}

func TestEnsureGeneratedFrom_InvalidYAML_ErrorAndUnchanged(t *testing.T) {
	t.Parallel()
	block := "kind: [unclosed\n"
	out, changed, err := EnsureGeneratedFrom(block, "req")
	assert.Error(t, err)
	assert.False(t, changed)
	assert.Equal(t, block, out)
}

func TestEnsureGeneratedFrom_TruncatesRequirementTo512Runes(t *testing.T) {
	t.Parallel()
	requirement := strings.Repeat("é", 600) // multi-byte runes prove UTF-8 safety
	out, changed, err := EnsureGeneratedFrom(unannotatedRGD, requirement)
	require.NoError(t, err)
	require.True(t, changed)

	parsed := parseRGD(t, out)
	resources := parsed["spec"].(map[string]interface{})["resources"].([]interface{})
	got := annotationOf(t, resources[0])
	assert.Equal(t, 512, len([]rune(got)))
	assert.Equal(t, strings.Repeat("é", 512), got)
}

func TestEnsureGeneratedFrom_PreservesKeyOrderAndComments(t *testing.T) {
	t.Parallel()
	block := `# header comment
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: ordered
spec:
  schema:
    kind: Ordered
    apiVersion: v1alpha1
  resources:
    - id: zeta
      template:
        kind: Deployment # inline comment
        apiVersion: apps/v1
`
	out, changed, err := EnsureGeneratedFrom(block, "req")
	require.NoError(t, err)
	require.True(t, changed)

	// Key order preserved: kind before apiVersion in both places it was
	// authored that way (a map round-trip would alphabetize/scramble).
	assert.Less(t, strings.Index(out, "kind: Ordered"), strings.Index(out, "apiVersion: v1alpha1"),
		"schema key order must survive the round-trip")
	assert.Less(t, strings.Index(out, "kind: Deployment"), strings.Index(out, "apiVersion: apps/v1"),
		"template key order must survive the round-trip")
	assert.Contains(t, out, "# header comment")
	assert.Contains(t, out, "# inline comment")
}

func TestBackfillResponse(t *testing.T) {
	t.Parallel()

	t.Run("backfills inside the fences, prose preserved verbatim", func(t *testing.T) {
		t.Parallel()
		text := "Intro prose.\n\n```yaml\n" + unannotatedRGD + "```\n\nOutro prose."
		got := BackfillResponse(text, "web app with redis")
		assert.True(t, strings.HasPrefix(got, "Intro prose.\n\n```yaml\n"))
		assert.True(t, strings.HasSuffix(got, "```\n\nOutro prose."))
		assert.Contains(t, got, GeneratedFromAnnotation)
		assert.Contains(t, got, "web app with redis")
	})

	t.Run("already annotated → text unchanged", func(t *testing.T) {
		t.Parallel()
		text := "Spec:\n```yaml\n" + annotatedRGD + "```\nDone."
		assert.Equal(t, text, BackfillResponse(text, "whole prompt"))
	})

	t.Run("no fenced block → text unchanged", func(t *testing.T) {
		t.Parallel()
		text := "No matching CRDs found for: Redis"
		assert.Equal(t, text, BackfillResponse(text, "req"))
	})

	t.Run("bare fence fallback is honored", func(t *testing.T) {
		t.Parallel()
		text := "```\n" + unannotatedRGD + "```"
		got := BackfillResponse(text, "req")
		assert.Contains(t, got, GeneratedFromAnnotation)
	})

	t.Run("invalid YAML → text unchanged", func(t *testing.T) {
		t.Parallel()
		text := "```yaml\nkind: [unclosed\n```"
		assert.Equal(t, text, BackfillResponse(text, "req"))
	})

	t.Run("non-RGD block → text unchanged", func(t *testing.T) {
		t.Parallel()
		text := "```yaml\nkind: Deployment\nspec:\n  replicas: 1\n```"
		assert.Equal(t, text, BackfillResponse(text, "req"))
	})
}

// parseRGD round-trips the emitted YAML through a plain unmarshal for
// structural assertions.
func parseRGD(t *testing.T, out string) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(out), &parsed), "backfilled output must stay valid YAML")
	return parsed
}

// annotationOf digs template.metadata.annotations[GeneratedFromAnnotation]
// out of a parsed resources[] entry.
func annotationOf(t *testing.T, resource interface{}) string {
	t.Helper()
	template, ok := resource.(map[string]interface{})["template"].(map[string]interface{})
	require.True(t, ok, "resource entry must have a template mapping")
	metadata, ok := template["metadata"].(map[string]interface{})
	require.True(t, ok, "template must have metadata")
	annotations, ok := metadata["annotations"].(map[string]interface{})
	require.True(t, ok, "metadata must have annotations")
	value, _ := annotations[GeneratedFromAnnotation].(string)
	return value
}
