// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetSpec returns the spec field of a Kubernetes object as a map.
// Returns ErrFieldNotFound if the spec doesn't exist.
// Returns ErrTypeMismatch if the spec exists but is not a map.
// Returns ErrNilObject if the object is nil.
func GetSpec(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	if obj == nil {
		return nil, newPathError("GetSpec", []string{"spec"}, "map[string]interface{}", "", ErrNilObject)
	}

	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, newPathError("GetSpec", []string{"spec"}, "map[string]interface{}", "", err)
	}
	if !found {
		return nil, newPathError("GetSpec", []string{"spec"}, "map[string]interface{}", "", ErrFieldNotFound)
	}

	return spec, nil
}

// GetSpecOrEmpty returns the spec field of a Kubernetes object as a map,
// or an empty map if the spec doesn't exist or is not a map.
func GetSpecOrEmpty(obj *unstructured.Unstructured) map[string]interface{} {
	spec, err := GetSpec(obj)
	if err != nil {
		return make(map[string]interface{})
	}
	return spec
}

// GetStatus returns the status field of a Kubernetes object as a map.
// Returns ErrFieldNotFound if the status doesn't exist.
// Returns ErrTypeMismatch if the status exists but is not a map.
// Returns ErrNilObject if the object is nil.
func GetStatus(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	if obj == nil {
		return nil, newPathError("GetStatus", []string{"status"}, "map[string]interface{}", "", ErrNilObject)
	}

	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return nil, newPathError("GetStatus", []string{"status"}, "map[string]interface{}", "", err)
	}
	if !found {
		return nil, newPathError("GetStatus", []string{"status"}, "map[string]interface{}", "", ErrFieldNotFound)
	}

	return status, nil
}

// GetStatusOrEmpty returns the status field of a Kubernetes object as a map,
// or an empty map if the status doesn't exist or is not a map.
func GetStatusOrEmpty(obj *unstructured.Unstructured) map[string]interface{} {
	status, err := GetStatus(obj)
	if err != nil {
		return make(map[string]interface{})
	}
	return status
}

// GetSpecField returns a field at the specified path within the spec.
// The path is relative to spec, so GetSpecField(obj, "template", "spec")
// returns obj.spec.template.spec.
// Returns ErrFieldNotFound if the spec or field doesn't exist.
// Returns ErrNilObject if the object is nil.
func GetSpecField(obj *unstructured.Unstructured, path ...string) (interface{}, error) {
	if obj == nil {
		fullPath := append([]string{"spec"}, path...)
		return nil, newPathError("GetSpecField", fullPath, "interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return GetSpec(obj)
	}

	fullPath := append([]string{"spec"}, path...)
	return GetValue(obj.Object, fullPath...)
}

// GetSpecFieldString returns a string field at the specified path within the spec.
func GetSpecFieldString(obj *unstructured.Unstructured, path ...string) (string, error) {
	if obj == nil {
		fullPath := append([]string{"spec"}, path...)
		return "", newPathError("GetSpecFieldString", fullPath, "string", "", ErrNilObject)
	}
	if len(path) == 0 {
		fullPath := []string{"spec"}
		return "", newPathError("GetSpecFieldString", fullPath, "string", "", ErrEmptyPath)
	}

	fullPath := append([]string{"spec"}, path...)
	return GetString(obj.Object, fullPath...)
}

// GetSpecFieldStringOrDefault returns a string field at the specified path
// within the spec, or a default value if not found.
func GetSpecFieldStringOrDefault(obj *unstructured.Unstructured, defaultVal string, path ...string) string {
	str, err := GetSpecFieldString(obj, path...)
	if err != nil {
		return defaultVal
	}
	return str
}

// GetSpecFieldMap returns a map field at the specified path within the spec.
func GetSpecFieldMap(obj *unstructured.Unstructured, path ...string) (map[string]interface{}, error) {
	if obj == nil {
		fullPath := append([]string{"spec"}, path...)
		return nil, newPathError("GetSpecFieldMap", fullPath, "map[string]interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return GetSpec(obj)
	}

	fullPath := append([]string{"spec"}, path...)
	return GetMap(obj.Object, fullPath...)
}

// GetSpecFieldSlice returns a slice field at the specified path within the spec.
func GetSpecFieldSlice(obj *unstructured.Unstructured, path ...string) ([]interface{}, error) {
	if obj == nil {
		fullPath := append([]string{"spec"}, path...)
		return nil, newPathError("GetSpecFieldSlice", fullPath, "[]interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		fullPath := []string{"spec"}
		return nil, newPathError("GetSpecFieldSlice", fullPath, "[]interface{}", "", ErrEmptyPath)
	}

	fullPath := append([]string{"spec"}, path...)
	return GetSlice(obj.Object, fullPath...)
}

// GetStatusField returns a field at the specified path within the status.
// The path is relative to status, so GetStatusField(obj, "conditions")
// returns obj.status.conditions.
// Returns ErrFieldNotFound if the status or field doesn't exist.
// Returns ErrNilObject if the object is nil.
func GetStatusField(obj *unstructured.Unstructured, path ...string) (interface{}, error) {
	if obj == nil {
		fullPath := append([]string{"status"}, path...)
		return nil, newPathError("GetStatusField", fullPath, "interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return GetStatus(obj)
	}

	fullPath := append([]string{"status"}, path...)
	return GetValue(obj.Object, fullPath...)
}

// GetStatusFieldString returns a string field at the specified path within the status.
func GetStatusFieldString(obj *unstructured.Unstructured, path ...string) (string, error) {
	if obj == nil {
		fullPath := append([]string{"status"}, path...)
		return "", newPathError("GetStatusFieldString", fullPath, "string", "", ErrNilObject)
	}
	if len(path) == 0 {
		fullPath := []string{"status"}
		return "", newPathError("GetStatusFieldString", fullPath, "string", "", ErrEmptyPath)
	}

	fullPath := append([]string{"status"}, path...)
	return GetString(obj.Object, fullPath...)
}

// GetStatusFieldStringOrDefault returns a string field at the specified path
// within the status, or a default value if not found.
func GetStatusFieldStringOrDefault(obj *unstructured.Unstructured, defaultVal string, path ...string) string {
	str, err := GetStatusFieldString(obj, path...)
	if err != nil {
		return defaultVal
	}
	return str
}

// GetStatusFieldMap returns a map field at the specified path within the status.
func GetStatusFieldMap(obj *unstructured.Unstructured, path ...string) (map[string]interface{}, error) {
	if obj == nil {
		fullPath := append([]string{"status"}, path...)
		return nil, newPathError("GetStatusFieldMap", fullPath, "map[string]interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		return GetStatus(obj)
	}

	fullPath := append([]string{"status"}, path...)
	return GetMap(obj.Object, fullPath...)
}

// GetStatusFieldSlice returns a slice field at the specified path within the status.
func GetStatusFieldSlice(obj *unstructured.Unstructured, path ...string) ([]interface{}, error) {
	if obj == nil {
		fullPath := append([]string{"status"}, path...)
		return nil, newPathError("GetStatusFieldSlice", fullPath, "[]interface{}", "", ErrNilObject)
	}
	if len(path) == 0 {
		fullPath := []string{"status"}
		return nil, newPathError("GetStatusFieldSlice", fullPath, "[]interface{}", "", ErrEmptyPath)
	}

	fullPath := append([]string{"status"}, path...)
	return GetSlice(obj.Object, fullPath...)
}

// HasSpec returns true if the object has a spec field.
func HasSpec(obj *unstructured.Unstructured) bool {
	_, err := GetSpec(obj)
	return err == nil
}

// HasStatus returns true if the object has a status field.
func HasStatus(obj *unstructured.Unstructured) bool {
	_, err := GetStatus(obj)
	return err == nil
}

// GetConditions returns the conditions slice from status.conditions.
// This is a common pattern in Kubernetes objects.
// Returns an empty slice if conditions don't exist.
func GetConditions(obj *unstructured.Unstructured) []interface{} {
	conditions, err := GetStatusFieldSlice(obj, "conditions")
	if err != nil {
		return []interface{}{}
	}
	return conditions
}

// GetCondition returns a specific condition by type from status.conditions.
// Returns nil if the condition is not found.
func GetCondition(obj *unstructured.Unstructured, conditionType string) map[string]interface{} {
	conditions := GetConditions(obj)
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, ok := condition["type"].(string); ok && t == conditionType {
			return condition
		}
	}
	return nil
}

// GetConditionStatus returns the status of a specific condition.
// Returns empty string if the condition doesn't exist.
func GetConditionStatus(obj *unstructured.Unstructured, conditionType string) string {
	condition := GetCondition(obj, conditionType)
	if condition == nil {
		return ""
	}
	status, _ := condition["status"].(string)
	return status
}

// IsConditionTrue returns true if a specific condition has status "True".
func IsConditionTrue(obj *unstructured.Unstructured, conditionType string) bool {
	return GetConditionStatus(obj, conditionType) == "True"
}

// IsConditionFalse returns true if a specific condition has status "False".
func IsConditionFalse(obj *unstructured.Unstructured, conditionType string) bool {
	return GetConditionStatus(obj, conditionType) == "False"
}

// GetConditionReason returns the reason of a specific condition.
// Returns empty string if the condition doesn't exist.
func GetConditionReason(obj *unstructured.Unstructured, conditionType string) string {
	condition := GetCondition(obj, conditionType)
	if condition == nil {
		return ""
	}
	reason, _ := condition["reason"].(string)
	return reason
}

// GetConditionMessage returns the message of a specific condition.
// Returns empty string if the condition doesn't exist.
func GetConditionMessage(obj *unstructured.Unstructured, conditionType string) string {
	condition := GetCondition(obj, conditionType)
	if condition == nil {
		return ""
	}
	message, _ := condition["message"].(string)
	return message
}
