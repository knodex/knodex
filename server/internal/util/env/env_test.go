package env

import (
	"testing"
	"time"
)

func TestGetString(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("TEST_STRING", "hello")
		if got := GetString("TEST_STRING", "default"); got != "hello" {
			t.Errorf("GetString() = %q, want %q", got, "hello")
		}
	})

	t.Run("not set", func(t *testing.T) {
		if got := GetString("TEST_STRING_MISSING", "default"); got != "default" {
			t.Errorf("GetString() = %q, want %q", got, "default")
		}
	})

	t.Run("empty value", func(t *testing.T) {
		t.Setenv("TEST_STRING_EMPTY", "")
		if got := GetString("TEST_STRING_EMPTY", "default"); got != "" {
			t.Errorf("GetString() = %q, want empty", got)
		}
	})
}

func TestGetInt(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("TEST_INT", "42")
		if got := GetInt("TEST_INT", 0); got != 42 {
			t.Errorf("GetInt() = %d, want 42", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("TEST_INT_BAD", "abc")
		if got := GetInt("TEST_INT_BAD", 10); got != 10 {
			t.Errorf("GetInt() = %d, want 10 (default)", got)
		}
	})

	t.Run("not set", func(t *testing.T) {
		if got := GetInt("TEST_INT_MISSING", 99); got != 99 {
			t.Errorf("GetInt() = %d, want 99 (default)", got)
		}
	})

	t.Run("negative", func(t *testing.T) {
		t.Setenv("TEST_INT_NEG", "-5")
		if got := GetInt("TEST_INT_NEG", 0); got != -5 {
			t.Errorf("GetInt() = %d, want -5", got)
		}
	})
}

func TestGetBool(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		t.Setenv("TEST_BOOL", "true")
		if got := GetBool("TEST_BOOL", false); got != true {
			t.Errorf("GetBool() = %v, want true", got)
		}
	})

	t.Run("false", func(t *testing.T) {
		t.Setenv("TEST_BOOL_F", "false")
		if got := GetBool("TEST_BOOL_F", true); got != false {
			t.Errorf("GetBool() = %v, want false", got)
		}
	})

	t.Run("1", func(t *testing.T) {
		t.Setenv("TEST_BOOL_1", "1")
		if got := GetBool("TEST_BOOL_1", false); got != true {
			t.Errorf("GetBool() = %v, want true", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("TEST_BOOL_BAD", "maybe")
		if got := GetBool("TEST_BOOL_BAD", true); got != true {
			t.Errorf("GetBool() = %v, want true (default)", got)
		}
	})

	t.Run("not set", func(t *testing.T) {
		if got := GetBool("TEST_BOOL_MISSING", true); got != true {
			t.Errorf("GetBool() = %v, want true (default)", got)
		}
	})
}

func TestGetStringSlice(t *testing.T) {
	t.Run("multiple", func(t *testing.T) {
		t.Setenv("TEST_SLICE", "a,b,c")
		got := GetStringSlice("TEST_SLICE")
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Errorf("GetStringSlice() = %v, want [a b c]", got)
		}
	})

	t.Run("with spaces", func(t *testing.T) {
		t.Setenv("TEST_SLICE_SPACES", " a , b , c ")
		got := GetStringSlice("TEST_SLICE_SPACES")
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Errorf("GetStringSlice() = %v, want [a b c]", got)
		}
	})

	t.Run("single", func(t *testing.T) {
		t.Setenv("TEST_SLICE_SINGLE", "only")
		got := GetStringSlice("TEST_SLICE_SINGLE")
		if len(got) != 1 || got[0] != "only" {
			t.Errorf("GetStringSlice() = %v, want [only]", got)
		}
	})

	t.Run("empty entries", func(t *testing.T) {
		t.Setenv("TEST_SLICE_EMPTY", "a,,b,")
		got := GetStringSlice("TEST_SLICE_EMPTY")
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("GetStringSlice() = %v, want [a b]", got)
		}
	})

	t.Run("not set", func(t *testing.T) {
		got := GetStringSlice("TEST_SLICE_MISSING")
		if got != nil {
			t.Errorf("GetStringSlice() = %v, want nil", got)
		}
	})
}

func TestGetDuration(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("TEST_DUR", "5s")
		if got := GetDuration("TEST_DUR", time.Second); got != 5*time.Second {
			t.Errorf("GetDuration() = %v, want 5s", got)
		}
	})

	t.Run("minutes", func(t *testing.T) {
		t.Setenv("TEST_DUR_MIN", "2m30s")
		if got := GetDuration("TEST_DUR_MIN", time.Second); got != 150*time.Second {
			t.Errorf("GetDuration() = %v, want 2m30s", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("TEST_DUR_BAD", "not-a-duration")
		if got := GetDuration("TEST_DUR_BAD", 10*time.Second); got != 10*time.Second {
			t.Errorf("GetDuration() = %v, want 10s (default)", got)
		}
	})

	t.Run("not set", func(t *testing.T) {
		if got := GetDuration("TEST_DUR_MISSING", 30*time.Second); got != 30*time.Second {
			t.Errorf("GetDuration() = %v, want 30s (default)", got)
		}
	})
}
