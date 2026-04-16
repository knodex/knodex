// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

// GetString retrieves a string value at the specified path in a nested map.
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrTypeMismatch if the value exists but is not a string.
// Returns ErrNilObject if obj is nil.
func GetString(obj map[string]interface{}, path ...string) (string, error) {
	if obj == nil {
		return "", newPathError("GetString", path, "string", "", ErrNilObject)
	}
	if len(path) == 0 {
		return "", newPathError("GetString", path, "string", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return "", newPathError("GetString", path, "string", "", err)
	}

	str, ok := val.(string)
	if !ok {
		return "", newPathError("GetString", path, "string", typeName(val), ErrTypeMismatch)
	}

	return str, nil
}

// GetStringOrDefault retrieves a string value at the specified path, returning
// defaultVal if the path doesn't exist or the value is not a string.
// This is useful when you want to gracefully handle missing or invalid fields.
func GetStringOrDefault(obj map[string]interface{}, defaultVal string, path ...string) string {
	str, err := GetString(obj, path...)
	if err != nil {
		return defaultVal
	}
	return str
}

// GetMap retrieves a map value at the specified path in a nested map.
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrTypeMismatch if the value exists but is not a map[string]interface{}.
// Returns ErrNilObject if obj is nil.
func GetMap(obj map[string]interface{}, path ...string) (map[string]interface{}, error) {
	if obj == nil {
		return nil, newPathError("GetMap", path, "map[string]interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return nil, newPathError("GetMap", path, "map[string]interface{}", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return nil, newPathError("GetMap", path, "map[string]interface{}", "", err)
	}

	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, newPathError("GetMap", path, "map[string]interface{}", typeName(val), ErrTypeMismatch)
	}

	return m, nil
}

// GetSlice retrieves a slice value at the specified path in a nested map.
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrTypeMismatch if the value exists but is not a []interface{}.
// Returns ErrNilObject if obj is nil.
func GetSlice(obj map[string]interface{}, path ...string) ([]interface{}, error) {
	if obj == nil {
		return nil, newPathError("GetSlice", path, "[]interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return nil, newPathError("GetSlice", path, "[]interface{}", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return nil, newPathError("GetSlice", path, "[]interface{}", "", err)
	}

	slice, ok := val.([]interface{})
	if !ok {
		return nil, newPathError("GetSlice", path, "[]interface{}", typeName(val), ErrTypeMismatch)
	}

	return slice, nil
}

// GetInt64 retrieves an int64 value at the specified path in a nested map.
// Handles both int64 and float64 (JSON numbers are parsed as float64).
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrTypeMismatch if the value exists but is not a numeric type.
// Returns ErrNilObject if obj is nil.
func GetInt64(obj map[string]interface{}, path ...string) (int64, error) {
	if obj == nil {
		return 0, newPathError("GetInt64", path, "int64", "", ErrNilObject)
	}
	if len(path) == 0 {
		return 0, newPathError("GetInt64", path, "int64", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return 0, newPathError("GetInt64", path, "int64", "", err)
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	default:
		return 0, newPathError("GetInt64", path, "int64", typeName(val), ErrTypeMismatch)
	}
}

// GetInt64OrDefault retrieves an int64 value at the specified path, returning
// defaultVal if the path doesn't exist or the value is not numeric.
func GetInt64OrDefault(obj map[string]interface{}, defaultVal int64, path ...string) int64 {
	n, err := GetInt64(obj, path...)
	if err != nil {
		return defaultVal
	}
	return n
}

// GetValue retrieves any value at the specified path in a nested map.
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrNilObject if obj is nil.
func GetValue(obj map[string]interface{}, path ...string) (interface{}, error) {
	if obj == nil {
		return nil, newPathError("GetValue", path, "interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return nil, newPathError("GetValue", path, "interface{}", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return nil, newPathError("GetValue", path, "interface{}", "", err)
	}

	return val, nil
}

// getNestedValue traverses a nested map structure and returns the value at the path.
func getNestedValue(obj map[string]interface{}, path []string) (interface{}, error) {
	current := interface{}(obj)

	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, ErrFieldNotFound
		}

		val, exists := m[key]
		if !exists {
			return nil, ErrFieldNotFound
		}

		current = val
	}

	return current, nil
}
