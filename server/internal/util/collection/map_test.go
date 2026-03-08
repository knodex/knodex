package collection

import (
	"sort"
	"testing"
)

func TestToSet(t *testing.T) {
	set := ToSet([]string{"a", "b", "c", "a"})
	if len(set) != 3 {
		t.Errorf("ToSet length = %d, want 3", len(set))
	}
	for _, k := range []string{"a", "b", "c"} {
		if !set[k] {
			t.Errorf("ToSet missing key %q", k)
		}
	}
}

func TestToSet_Empty(t *testing.T) {
	set := ToSet([]string{})
	if len(set) != 0 {
		t.Errorf("ToSet(empty) = %v, want empty", set)
	}
}

func TestToSet_Nil(t *testing.T) {
	set := ToSet[string](nil)
	if set == nil {
		t.Error("ToSet(nil) should return empty map, not nil")
	}
	if len(set) != 0 {
		t.Errorf("ToSet(nil) should have 0 elements, got %d", len(set))
	}
}

func TestKeys(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := Keys(m)
	sort.Strings(keys) // Order not guaranteed
	expected := []string{"a", "b", "c"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Keys()[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestKeys_Empty(t *testing.T) {
	keys := Keys(map[string]int{})
	if len(keys) != 0 {
		t.Errorf("Keys(empty) = %v, want empty", keys)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	keys := SortedKeys(m)
	expected := []string{"a", "b", "c"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("SortedKeys()[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	keys := SortedKeys(map[string]int{})
	if len(keys) != 0 {
		t.Errorf("SortedKeys(empty) = %v, want empty", keys)
	}
}

func TestValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	vals := Values(m)
	sort.Ints(vals) // Order not guaranteed
	expected := []int{1, 2}
	for i, v := range vals {
		if v != expected[i] {
			t.Errorf("Values()[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestValues_Empty(t *testing.T) {
	vals := Values(map[string]int{})
	if len(vals) != 0 {
		t.Errorf("Values(empty) = %v, want empty", vals)
	}
}

func TestSortedKeys_BoolMap(t *testing.T) {
	m := map[string]bool{"z": true, "a": true, "m": false}
	keys := SortedKeys(m)
	expected := []string{"a", "m", "z"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("SortedKeys()[%d] = %q, want %q", i, k, expected[i])
		}
	}
}
