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

// GetMapOrDefault retrieves a map value at the specified path, returning
// defaultVal if the path doesn't exist or the value is not a map.
func GetMapOrDefault(obj map[string]interface{}, defaultVal map[string]interface{}, path ...string) map[string]interface{} {
	m, err := GetMap(obj, path...)
	if err != nil {
		return defaultVal
	}
	return m
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

// GetSliceOrDefault retrieves a slice value at the specified path, returning
// defaultVal if the path doesn't exist or the value is not a slice.
func GetSliceOrDefault(obj map[string]interface{}, defaultVal []interface{}, path ...string) []interface{} {
	slice, err := GetSlice(obj, path...)
	if err != nil {
		return defaultVal
	}
	return slice
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

// GetBool retrieves a bool value at the specified path in a nested map.
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrTypeMismatch if the value exists but is not a bool.
// Returns ErrNilObject if obj is nil.
func GetBool(obj map[string]interface{}, path ...string) (bool, error) {
	if obj == nil {
		return false, newPathError("GetBool", path, "bool", "", ErrNilObject)
	}
	if len(path) == 0 {
		return false, newPathError("GetBool", path, "bool", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return false, newPathError("GetBool", path, "bool", "", err)
	}

	b, ok := val.(bool)
	if !ok {
		return false, newPathError("GetBool", path, "bool", typeName(val), ErrTypeMismatch)
	}

	return b, nil
}

// GetBoolOrDefault retrieves a bool value at the specified path, returning
// defaultVal if the path doesn't exist or the value is not a bool.
func GetBoolOrDefault(obj map[string]interface{}, defaultVal bool, path ...string) bool {
	b, err := GetBool(obj, path...)
	if err != nil {
		return defaultVal
	}
	return b
}

// GetFloat64 retrieves a float64 value at the specified path in a nested map.
// Returns ErrFieldNotFound if any part of the path doesn't exist.
// Returns ErrTypeMismatch if the value exists but is not a numeric type.
// Returns ErrNilObject if obj is nil.
func GetFloat64(obj map[string]interface{}, path ...string) (float64, error) {
	if obj == nil {
		return 0, newPathError("GetFloat64", path, "float64", "", ErrNilObject)
	}
	if len(path) == 0 {
		return 0, newPathError("GetFloat64", path, "float64", "", ErrEmptyPath)
	}

	val, err := getNestedValue(obj, path)
	if err != nil {
		return 0, newPathError("GetFloat64", path, "float64", "", err)
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	default:
		return 0, newPathError("GetFloat64", path, "float64", typeName(val), ErrTypeMismatch)
	}
}

// GetFloat64OrDefault retrieves a float64 value at the specified path, returning
// defaultVal if the path doesn't exist or the value is not numeric.
func GetFloat64OrDefault(obj map[string]interface{}, defaultVal float64, path ...string) float64 {
	f, err := GetFloat64(obj, path...)
	if err != nil {
		return defaultVal
	}
	return f
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

// HasField returns true if a field exists at the specified path.
func HasField(obj map[string]interface{}, path ...string) bool {
	_, err := GetValue(obj, path...)
	return err == nil
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

// SetNestedValue sets a value at the specified path in a nested map, creating
// intermediate maps as needed. Returns an error if the path is empty or if
// an intermediate value exists but is not a map.
func SetNestedValue(obj map[string]interface{}, value interface{}, path ...string) error {
	if obj == nil {
		return newPathError("SetNestedValue", path, "", "", ErrNilObject)
	}
	if len(path) == 0 {
		return newPathError("SetNestedValue", path, "", "", ErrEmptyPath)
	}

	current := obj
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		val, exists := current[key]

		if !exists {
			// Create intermediate map
			newMap := make(map[string]interface{})
			current[key] = newMap
			current = newMap
		} else {
			// Traverse existing map
			m, ok := val.(map[string]interface{})
			if !ok {
				return newPathError("SetNestedValue", path[:i+1], "map[string]interface{}", typeName(val), ErrTypeMismatch)
			}
			current = m
		}
	}

	current[path[len(path)-1]] = value
	return nil
}

// DeleteNestedValue removes a value at the specified path in a nested map.
// Returns true if the value was deleted, false if the path didn't exist.
func DeleteNestedValue(obj map[string]interface{}, path ...string) bool {
	if obj == nil || len(path) == 0 {
		return false
	}

	current := obj
	for i := 0; i < len(path)-1; i++ {
		val, exists := current[path[i]]
		if !exists {
			return false
		}
		m, ok := val.(map[string]interface{})
		if !ok {
			return false
		}
		current = m
	}

	key := path[len(path)-1]
	if _, exists := current[key]; !exists {
		return false
	}

	delete(current, key)
	return true
}
