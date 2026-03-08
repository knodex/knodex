package collection

import (
	"strings"
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty", []string{}, "a", false},
		{"nil", nil, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.slice, tt.item); got != tt.want {
				t.Errorf("Contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func TestContains_Int(t *testing.T) {
	if !Contains([]int{1, 2, 3}, 2) {
		t.Error("Contains([]int{1,2,3}, 2) should be true")
	}
	if Contains([]int{1, 2, 3}, 4) {
		t.Error("Contains([]int{1,2,3}, 4) should be false")
	}
}

func TestContainsFunc(t *testing.T) {
	type item struct {
		Name string
	}
	items := []item{{Name: "alpha"}, {Name: "beta"}}

	if !ContainsFunc(items, func(i item) bool { return i.Name == "alpha" }) {
		t.Error("expected to find alpha")
	}
	if ContainsFunc(items, func(i item) bool { return i.Name == "gamma" }) {
		t.Error("should not find gamma")
	}
	if ContainsFunc([]item{}, func(i item) bool { return true }) {
		t.Error("empty slice should return false")
	}
}

func TestFilter(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5, 6}
	evens := Filter(nums, func(n int) bool { return n%2 == 0 })

	if len(evens) != 3 {
		t.Errorf("expected 3 evens, got %d", len(evens))
	}
	for _, n := range evens {
		if n%2 != 0 {
			t.Errorf("expected even, got %d", n)
		}
	}
}

func TestFilter_Empty(t *testing.T) {
	result := Filter([]int{}, func(n int) bool { return true })
	if result == nil {
		t.Error("Filter should return non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice for empty input, got %v", result)
	}
}

func TestFilter_NoMatch(t *testing.T) {
	result := Filter([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 })
	if result == nil {
		t.Error("Filter should return non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice for no matches, got %v", result)
	}
}

func TestMap(t *testing.T) {
	nums := []int{1, 2, 3}
	strs := Map(nums, func(n int) string { return strings.Repeat("x", n) })

	expected := []string{"x", "xx", "xxx"}
	for i, s := range strs {
		if s != expected[i] {
			t.Errorf("Map result[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestMap_Nil(t *testing.T) {
	var input []int
	result := Map(input, func(n int) string { return "" })
	if result != nil {
		t.Errorf("Map(nil) should return nil, got %v", result)
	}
}

func TestDeduplicate(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"all same", []string{"a", "a", "a"}, []string{"a"}},
		{"empty", []string{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Deduplicate(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("Deduplicate(%v) length = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("Deduplicate(%v)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"same order", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different order", []string{"a", "b"}, []string{"b", "a"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different elements", []string{"a", "b"}, []string{"a", "c"}, false},
		{"with duplicates equal", []string{"a", "a", "b"}, []string{"b", "a", "a"}, true},
		{"with duplicates unequal", []string{"a", "a", "b"}, []string{"a", "b", "b"}, false},
		{"both empty", []string{}, []string{}, true},
		{"both nil", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.a, tt.b); got != tt.want {
				t.Errorf("Equal(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name        string
		old, new    []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			"some changes",
			[]string{"a", "b", "c"},
			[]string{"b", "c", "d"},
			[]string{"d"},
			[]string{"a"},
		},
		{
			"no changes",
			[]string{"a", "b"},
			[]string{"a", "b"},
			nil,
			nil,
		},
		{
			"all new",
			[]string{},
			[]string{"a", "b"},
			[]string{"a", "b"},
			nil,
		},
		{
			"all removed",
			[]string{"a", "b"},
			[]string{},
			nil,
			[]string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := Diff(tt.old, tt.new)
			if !equalOrBothNil(added, tt.wantAdded) {
				t.Errorf("Diff() added = %v, want %v", added, tt.wantAdded)
			}
			if !equalOrBothNil(removed, tt.wantRemoved) {
				t.Errorf("Diff() removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}
}

// equalOrBothNil compares two slices, treating nil and empty as distinct.
func equalOrBothNil(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
