package hash

import (
	"testing"
)

func TestSHA256(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			"empty",
			[]byte{},
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			"hello",
			[]byte("hello"),
			"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SHA256(tt.data); got != tt.want {
				t.Errorf("SHA256(%q) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestSHA256_Deterministic(t *testing.T) {
	data := []byte("deterministic test input")
	h1 := SHA256(data)
	h2 := SHA256(data)
	if h1 != h2 {
		t.Errorf("SHA256 not deterministic: %q != %q", h1, h2)
	}
}

func TestSHA256_Length(t *testing.T) {
	h := SHA256([]byte("test"))
	if len(h) != 64 {
		t.Errorf("SHA256 hash length = %d, want 64", len(h))
	}
}

func TestSHA256String(t *testing.T) {
	got := SHA256String("hello")
	want := SHA256([]byte("hello"))
	if got != want {
		t.Errorf("SHA256String and SHA256 disagree: %q != %q", got, want)
	}
}

func TestSHA256String_Empty(t *testing.T) {
	got := SHA256String("")
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("SHA256String(\"\") = %q, want %q", got, want)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		hash   string
		length int
		want   string
	}{
		{"truncate to 8", "abcdef1234567890", 8, "abcdef12"},
		{"truncate to 12", "abcdef1234567890", 12, "abcdef123456"},
		{"same length", "abcd", 4, "abcd"},
		{"shorter than length", "abc", 10, "abc"},
		{"zero length", "abc", 0, ""},
		{"negative length", "abc", -1, ""},
		{"empty hash", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.hash, tt.length); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.hash, tt.length, got, tt.want)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	// Should be deterministic
	h1 := ContentHash("part1", "part2", "part3")
	h2 := ContentHash("part1", "part2", "part3")
	if h1 != h2 {
		t.Errorf("ContentHash not deterministic: %q != %q", h1, h2)
	}

	// Different parts should produce different hashes
	h3 := ContentHash("part1", "part2")
	if h1 == h3 {
		t.Error("ContentHash with different parts should produce different hashes")
	}
}

func TestContentHash_Empty(t *testing.T) {
	h := ContentHash()
	want := SHA256String("")
	if h != want {
		t.Errorf("ContentHash() = %q, want %q", h, want)
	}
}

func TestContentHash_SinglePart(t *testing.T) {
	h := ContentHash("hello")
	want := SHA256String("hello")
	if h != want {
		t.Errorf("ContentHash('hello') = %q, want %q", h, want)
	}
}

func BenchmarkSHA256(b *testing.B) {
	data := []byte("benchmark test data for sha256 hashing")
	for b.Loop() {
		SHA256(data)
	}
}

func BenchmarkContentHash(b *testing.B) {
	for b.Loop() {
		ContentHash("part1", "part2", "part3")
	}
}
