package collection

import "slices"

// ToSet converts a slice to a map[T]bool set.
func ToSet[T comparable](slice []T) map[T]bool {
	m := make(map[T]bool, len(slice))
	for _, v := range slice {
		m[v] = true
	}
	return m
}

// Keys returns the keys of a map as a slice. Order is not guaranteed.
func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// SortedKeys returns the keys of a map sorted in ascending order.
// Works with any string-underlying key type (string, custom string types).
func SortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Values returns the values of a map as a slice. Order is not guaranteed.
func Values[K comparable, V any](m map[K]V) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}
