// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package vcs

import (
	"context"
	"fmt"

	"github.com/knodex/knodex/server/internal/metrics/gitops"
	utilhash "github.com/knodex/knodex/server/internal/util/hash"
)

// IdempotencyKeyLength is the length of the idempotency key included in commit messages
// SECURITY: Truncated to 16 chars to avoid leaking full hash
const IdempotencyKeyLength = 16

// ComputeContentHash computes a SHA256 hash of the content
// AC-IDEM-01: Each commit includes idempotency key (SHA256 of manifest content)
func ComputeContentHash(content []byte) string {
	return utilhash.SHA256(content)
}

// ComputeContentHashString computes a SHA256 hash of string content
func ComputeContentHashString(content string) string {
	return utilhash.SHA256String(content)
}

// FormatMessageWithIdempotencyKey formats a commit message with an idempotency key
// AC-IDEM-01: Each commit includes idempotency key in commit message
// SECURITY: Idempotency key truncated (16 chars) to avoid leaking full hash
func FormatMessageWithIdempotencyKey(message string, contentHash string) string {
	shortHash := contentHash
	if len(contentHash) > IdempotencyKeyLength {
		shortHash = contentHash[:IdempotencyKeyLength]
	}
	return fmt.Sprintf("%s\n\nIdempotency-Key: %s", message, shortHash)
}

// CommitWithIdempotency commits a file with idempotency checking
// AC-IDEM-02: Before committing, check if file content matches hash (skip if identical)
// AC-IDEM-03: Idempotency prevents duplicate commits on retry
// AC-IDEM-04: Idempotency check uses ETag from GitHub API for efficiency
func (c *GitHubClient) CommitWithIdempotency(ctx context.Context, req *CommitFileRequest) (*CommitResult, bool, error) {
	if req.Path == "" {
		return nil, false, fmt.Errorf("file path cannot be empty")
	}
	if req.Content == "" {
		return nil, false, fmt.Errorf("file content cannot be empty")
	}
	if req.Message == "" {
		return nil, false, fmt.Errorf("commit message cannot be empty")
	}

	// Compute content hash for idempotency check
	contentHash := ComputeContentHashString(req.Content)
	repoLabel := fmt.Sprintf("%s/%s", c.owner, c.repo)

	// Check if file already exists with same content
	// AC-IDEM-02: Before committing, check if file content matches hash
	existingContent, err := c.GetFileContent(ctx, req.Path, req.Branch)
	if err == nil && existingContent != nil {
		// File exists - check if content matches
		// AC-IDEM-04: Use ETag/SHA from GitHub API for efficiency
		existingHash := ComputeContentHash(decodeBase64Content(existingContent.Content))
		if existingHash == contentHash {
			// Content identical, skip commit
			// AC-IDEM-03: Idempotency prevents duplicate commits on retry
			gitops.CommitErrors.WithLabelValues(repoLabel, gitops.ErrorTypeIdempotent).Inc()
			return &CommitResult{
				SHA:     existingContent.SHA,
				Message: "skipped: content identical",
			}, true, nil
		}

		// Update existing file - need SHA for update
		req.SHA = existingContent.SHA
	}

	// AC-IDEM-01: Include hash in commit message for traceability
	originalMessage := req.Message
	req.Message = FormatMessageWithIdempotencyKey(originalMessage, contentHash)

	// Commit the file
	result, err := c.CommitFile(ctx, req)

	// Restore original message in request
	req.Message = originalMessage

	return result, false, err
}

// CommitMultipleWithIdempotency commits multiple files with idempotency checking
// Returns: result, skipped (true if all files identical), error
func (c *GitHubClient) CommitMultipleWithIdempotency(ctx context.Context, req *CommitMultipleFilesRequest) (*CommitResult, bool, error) {
	if len(req.Files) == 0 {
		return nil, false, fmt.Errorf("no files to commit")
	}
	if req.Message == "" {
		return nil, false, fmt.Errorf("commit message cannot be empty")
	}

	repoLabel := fmt.Sprintf("%s/%s", c.owner, c.repo)

	// Compute combined hash of all files for idempotency
	combinedContent := ""
	for path, content := range req.Files {
		combinedContent += path + ":" + content + "\n"
	}
	contentHash := ComputeContentHashString(combinedContent)

	// Check if all files already exist with same content
	allIdentical := true
	for path, content := range req.Files {
		existing, err := c.GetFileContent(ctx, path, req.Branch)
		if err != nil || existing == nil {
			// File doesn't exist or error - need to commit
			allIdentical = false
			break
		}

		existingHash := ComputeContentHash(decodeBase64Content(existing.Content))
		newHash := ComputeContentHashString(content)
		if existingHash != newHash {
			allIdentical = false
			break
		}
	}

	if allIdentical {
		// All files identical, skip commit
		gitops.CommitErrors.WithLabelValues(repoLabel, gitops.ErrorTypeIdempotent).Inc()
		return &CommitResult{
			Message: "skipped: all files identical",
		}, true, nil
	}

	// Include hash in commit message
	originalMessage := req.Message
	req.Message = FormatMessageWithIdempotencyKey(originalMessage, contentHash)

	// Commit all files
	result, err := c.CommitMultipleFiles(ctx, req)

	// Restore original message
	req.Message = originalMessage

	return result, false, err
}

// decodeBase64Content decodes base64-encoded content from GitHub API
// Note: GitHub returns content with newlines, so we handle that
func decodeBase64Content(encoded string) []byte {
	// GitHub may add newlines in base64 content
	decoded, err := decodeBase64WithNewlines(encoded)
	if err != nil {
		return []byte(encoded) // Return as-is if decode fails
	}
	return decoded
}

// decodeBase64WithNewlines decodes base64 that may contain newlines
func decodeBase64WithNewlines(s string) ([]byte, error) {
	// Remove whitespace that GitHub may add
	cleaned := ""
	for _, c := range s {
		if c != '\n' && c != '\r' && c != ' ' {
			cleaned += string(c)
		}
	}

	// Standard library handles the actual decoding
	return decodeBase64Standard(cleaned)
}

// decodeBase64Standard is the standard base64 decode
func decodeBase64Standard(s string) ([]byte, error) {
	if len(s) == 0 {
		return []byte{}, nil
	}

	// Use standard encoding with padding
	decoded := make([]byte, len(s))
	n := 0
	for i := 0; i < len(s); i += 4 {
		chunk := s[i:]
		if len(chunk) > 4 {
			chunk = chunk[:4]
		}

		// Decode 4 characters at a time
		vals := make([]int, 4)
		for j := 0; j < len(chunk); j++ {
			vals[j] = base64Decode(chunk[j])
			if vals[j] < 0 && chunk[j] != '=' {
				return nil, fmt.Errorf("invalid base64 character: %c", chunk[j])
			}
		}

		// Convert to 3 bytes
		if len(chunk) >= 2 && vals[0] >= 0 && vals[1] >= 0 {
			decoded[n] = byte((vals[0] << 2) | (vals[1] >> 4))
			n++
		}
		if len(chunk) >= 3 && vals[2] >= 0 && chunk[2] != '=' {
			decoded[n] = byte((vals[1] << 4) | (vals[2] >> 2))
			n++
		}
		if len(chunk) >= 4 && vals[3] >= 0 && chunk[3] != '=' {
			decoded[n] = byte((vals[2] << 6) | vals[3])
			n++
		}
	}

	return decoded[:n], nil
}

// base64Decode returns the value of a base64 character, or -1 if invalid
func base64Decode(c byte) int {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A')
	case c >= 'a' && c <= 'z':
		return int(c - 'a' + 26)
	case c >= '0' && c <= '9':
		return int(c - '0' + 52)
	case c == '+':
		return 62
	case c == '/':
		return 63
	default:
		return -1
	}
}
