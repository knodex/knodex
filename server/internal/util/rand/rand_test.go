package rand

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		wantLen int
		wantErr bool
	}{
		{"zero bytes", 0, 0, false},
		{"one byte", 1, 1, false},
		{"16 bytes", 16, 16, false},
		{"32 bytes", 32, 32, false},
		{"negative bytes", -1, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateRandomBytes(tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRandomBytes(%d) error = %v, wantErr %v", tt.n, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("GenerateRandomBytes(%d) returned %d bytes, want %d", tt.n, len(got), tt.wantLen)
			}
		})
	}
}

func TestGenerateRandomBytes_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		b, err := GenerateRandomBytes(16)
		if err != nil {
			t.Fatalf("GenerateRandomBytes(16) failed: %v", err)
		}
		key := hex.EncodeToString(b)
		if seen[key] {
			t.Fatalf("GenerateRandomBytes(16) produced duplicate value on iteration %d", i)
		}
		seen[key] = true
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name      string
		n         int
		wantChars int // expected base64 string length
	}{
		{"16 bytes", 16, 24}, // base64(16 bytes) = 24 chars (with padding)
		{"32 bytes", 32, 44}, // base64(32 bytes) = 44 chars (with padding)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateRandomString(tt.n)
			if len(got) != tt.wantChars {
				t.Errorf("GenerateRandomString(%d) length = %d, want %d", tt.n, len(got), tt.wantChars)
			}

			// Verify it's valid base64
			decoded, err := base64.URLEncoding.DecodeString(got)
			if err != nil {
				t.Errorf("GenerateRandomString(%d) produced invalid base64: %v", tt.n, err)
			}
			if len(decoded) != tt.n {
				t.Errorf("GenerateRandomString(%d) decoded to %d bytes, want %d", tt.n, len(decoded), tt.n)
			}
		})
	}
}

func TestGenerateRandomString_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		s := GenerateRandomString(32)
		if seen[s] {
			t.Fatalf("GenerateRandomString(32) produced duplicate on iteration %d", i)
		}
		seen[s] = true
	}
}

func TestGenerateRandomHex(t *testing.T) {
	tests := []struct {
		name      string
		n         int
		wantChars int // hex string length = 2 * n
	}{
		{"4 bytes", 4, 8},
		{"16 bytes", 16, 32},
		{"32 bytes", 32, 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateRandomHex(tt.n)
			if len(got) != tt.wantChars {
				t.Errorf("GenerateRandomHex(%d) length = %d, want %d", tt.n, len(got), tt.wantChars)
			}

			// Verify it's valid hex
			decoded, err := hex.DecodeString(got)
			if err != nil {
				t.Errorf("GenerateRandomHex(%d) produced invalid hex: %v", tt.n, err)
			}
			if len(decoded) != tt.n {
				t.Errorf("GenerateRandomHex(%d) decoded to %d bytes, want %d", tt.n, len(decoded), tt.n)
			}
		})
	}
}

func TestGenerateRandomHex_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		s := GenerateRandomHex(16)
		if seen[s] {
			t.Fatalf("GenerateRandomHex(16) produced duplicate on iteration %d", i)
		}
		seen[s] = true
	}
}

func TestGenerateRandomHex_CharacterSet(t *testing.T) {
	s := GenerateRandomHex(32)
	for i, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateRandomHex(32) contains non-hex character %q at position %d", c, i)
		}
	}
}

func TestGenerateToken(t *testing.T) {
	// GenerateToken should behave identically to GenerateRandomString
	token := GenerateToken(32)

	// Verify it's valid base64 URL encoding
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("GenerateToken(32) produced invalid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("GenerateToken(32) decoded to %d bytes, want 32", len(decoded))
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		s := GenerateToken(32)
		if seen[s] {
			t.Fatalf("GenerateToken(32) produced duplicate on iteration %d", i)
		}
		seen[s] = true
	}
}

// failReader always returns an error to simulate crypto/rand failure.
type failReader struct{}

func (failReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated rand failure")
}

// withFailingReader replaces the package reader with a failing one for the
// duration of the test, restoring it afterward.
func withFailingReader(t *testing.T) {
	t.Helper()
	orig := reader
	reader = failReader{}
	t.Cleanup(func() { reader = orig })
}

func TestGenerateRandomBytes_ReaderFailure(t *testing.T) {
	withFailingReader(t)

	_, err := GenerateRandomBytes(16)
	if err == nil {
		t.Fatal("GenerateRandomBytes should return error when reader fails")
	}
	if !strings.Contains(err.Error(), "failed to generate") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerateRandomString_PanicsOnFailure(t *testing.T) {
	withFailingReader(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("GenerateRandomString should panic when reader fails")
		}
	}()
	GenerateRandomString(16)
}

func TestGenerateRandomHex_PanicsOnFailure(t *testing.T) {
	withFailingReader(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("GenerateRandomHex should panic when reader fails")
		}
	}()
	GenerateRandomHex(16)
}

func TestGenerateToken_PanicsOnFailure(t *testing.T) {
	withFailingReader(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("GenerateToken should panic when reader fails")
		}
	}()
	GenerateToken(16)
}

func TestReaderDefaultsToCryptoRand(t *testing.T) {
	// Verify the default reader is crypto/rand.Reader
	if reader != io.Reader(cryptorand.Reader) {
		t.Error("default reader should be crypto/rand.Reader")
	}
}

func BenchmarkGenerateRandomBytes(b *testing.B) {
	for b.Loop() {
		GenerateRandomBytes(32)
	}
}

func BenchmarkGenerateRandomString(b *testing.B) {
	for b.Loop() {
		GenerateRandomString(32)
	}
}

func BenchmarkGenerateRandomHex(b *testing.B) {
	for b.Loop() {
		GenerateRandomHex(16)
	}
}
