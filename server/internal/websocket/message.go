package websocket

import (
	"encoding/json"
	"time"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
	// Server -> Client message types
	MessageTypeInstanceUpdate   MessageType = "instance_update"
	MessageTypeRGDUpdate        MessageType = "rgd_update"
	MessageTypeViolationUpdate  MessageType = "violation_update"
	MessageTypeTemplateUpdate   MessageType = "template_update"   // Enterprise: ConstraintTemplate changes
	MessageTypeConstraintUpdate MessageType = "constraint_update" // Enterprise: Constraint changes
	MessageTypeCountsUpdate     MessageType = "counts_update"     // Sidebar badge count push
	MessageTypeError            MessageType = "error"
	MessageTypePong             MessageType = "pong"
	MessageTypeSubscribed       MessageType = "subscribed"
	MessageTypeUnsubscribed     MessageType = "unsubscribed"

	// Client -> Server message types
	MessageTypeSubscribe   MessageType = "subscribe"
	MessageTypeUnsubscribe MessageType = "unsubscribe"
	MessageTypePing        MessageType = "ping"
)

// Action defines the action performed on a resource
type Action string

const (
	ActionAdd    Action = "add"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// Message is the base WebSocket message structure
type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// InstanceUpdateData contains instance update information
type InstanceUpdateData struct {
	Action    Action      `json:"action"`
	Namespace string      `json:"namespace"`
	Name      string      `json:"name"`
	Instance  interface{} `json:"instance,omitempty"`
	ProjectID string      `json:"projectId,omitempty"` // For project-scoped filtering
}

// RGDUpdateData contains RGD update information
type RGDUpdateData struct {
	Action    Action      `json:"action"`
	Name      string      `json:"name"`
	RGD       interface{} `json:"rgd,omitempty"`
	ProjectID string      `json:"projectId,omitempty"` // For project-scoped filtering
}

// ViolationUpdateData contains violation update information for OPA Gatekeeper compliance.
// This is an enterprise-only feature.
type ViolationUpdateData struct {
	// Action indicates whether the violation was detected or resolved
	Action Action `json:"action"`

	// ConstraintKind is the kind of constraint that was violated (e.g., K8sRequiredLabels)
	ConstraintKind string `json:"constraintKind"`

	// ConstraintName is the name of the constraint that was violated
	ConstraintName string `json:"constraintName"`

	// Resource identifies the Kubernetes resource that has the violation
	Resource ViolationResourceData `json:"resource"`

	// Message is the human-readable violation message from the Rego policy
	Message string `json:"message"`

	// EnforcementAction specifies what happens on violation: deny, dryrun, or warn
	EnforcementAction string `json:"enforcementAction"`
}

// ViolationResourceData identifies a Kubernetes resource in a violation event
type ViolationResourceData struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	APIGroup  string `json:"apiGroup,omitempty"`
}

// TemplateUpdateData contains constraint template update information for OPA Gatekeeper compliance.
// This is an enterprise-only feature.
type TemplateUpdateData struct {
	// Action indicates whether the template was added, updated, or removed
	Action Action `json:"action"`

	// Name is the name of the ConstraintTemplate
	Name string `json:"name"`

	// Kind is the constraint kind produced by this template (e.g., K8sRequiredLabels)
	Kind string `json:"kind"`

	// Description is a human-readable description of the template
	Description string `json:"description,omitempty"`
}

// ConstraintUpdateData contains constraint update information for OPA Gatekeeper compliance.
// This is an enterprise-only feature.
type ConstraintUpdateData struct {
	// Action indicates whether the constraint was added, updated, or removed
	Action Action `json:"action"`

	// Kind is the constraint kind (determined by the ConstraintTemplate)
	Kind string `json:"kind"`

	// Name is the name of the constraint
	Name string `json:"name"`

	// EnforcementAction specifies what happens on violation: deny, dryrun, or warn
	EnforcementAction string `json:"enforcementAction"`

	// ViolationCount is the current number of violations
	ViolationCount int `json:"violationCount"`
}

// CountsUpdateData contains sidebar badge count data pushed to clients
type CountsUpdateData struct {
	RGDCount      int `json:"rgdCount"`
	InstanceCount int `json:"instanceCount"`
}

// ErrorData contains error information
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SubscribeData contains subscription request data
type SubscribeData struct {
	// Resource type: "instance", "rgd", "all"
	ResourceType string `json:"resourceType"`
	// Optional namespace filter (for instances)
	Namespace string `json:"namespace,omitempty"`
	// Optional name filter
	Name string `json:"name,omitempty"`
}

// SubscriptionConfirmData confirms a subscription
type SubscriptionConfirmData struct {
	ResourceType string `json:"resourceType"`
	Namespace    string `json:"namespace,omitempty"`
	Name         string `json:"name,omitempty"`
	Success      bool   `json:"success"`
}

// NewMessage creates a new message with the given type and data
func NewMessage(msgType MessageType, data interface{}) (*Message, error) {
	var rawData json.RawMessage
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = bytes
	}

	return &Message{
		Type:      msgType,
		Timestamp: time.Now().UTC(),
		Data:      rawData,
	}, nil
}

// NewInstanceUpdateMessage creates an instance update message
func NewInstanceUpdateMessage(action Action, namespace, name string, instance interface{}, projectID string) (*Message, error) {
	data := InstanceUpdateData{
		Action:    action,
		Namespace: namespace,
		Name:      name,
		Instance:  instance,
		ProjectID: projectID,
	}
	return NewMessage(MessageTypeInstanceUpdate, data)
}

// NewRGDUpdateMessage creates an RGD update message
func NewRGDUpdateMessage(action Action, name string, rgd interface{}, projectID string) (*Message, error) {
	data := RGDUpdateData{
		Action:    action,
		Name:      name,
		RGD:       rgd,
		ProjectID: projectID,
	}
	return NewMessage(MessageTypeRGDUpdate, data)
}

// NewViolationUpdateMessage creates a violation update message for OPA Gatekeeper compliance.
// action should be ActionAdd for newly detected violations, or ActionDelete for resolved violations.
func NewViolationUpdateMessage(action Action, constraintKind, constraintName string, resource ViolationResourceData, message, enforcementAction string) (*Message, error) {
	data := ViolationUpdateData{
		Action:            action,
		ConstraintKind:    constraintKind,
		ConstraintName:    constraintName,
		Resource:          resource,
		Message:           message,
		EnforcementAction: enforcementAction,
	}
	return NewMessage(MessageTypeViolationUpdate, data)
}

// NewTemplateUpdateMessage creates a constraint template update message for OPA Gatekeeper compliance.
// action should be ActionAdd, ActionUpdate, or ActionDelete.
func NewTemplateUpdateMessage(action Action, name, kind, description string) (*Message, error) {
	data := TemplateUpdateData{
		Action:      action,
		Name:        name,
		Kind:        kind,
		Description: description,
	}
	return NewMessage(MessageTypeTemplateUpdate, data)
}

// NewConstraintUpdateMessage creates a constraint update message for OPA Gatekeeper compliance.
// action should be ActionAdd, ActionUpdate, or ActionDelete.
func NewConstraintUpdateMessage(action Action, kind, name, enforcementAction string, violationCount int) (*Message, error) {
	data := ConstraintUpdateData{
		Action:            action,
		Kind:              kind,
		Name:              name,
		EnforcementAction: enforcementAction,
		ViolationCount:    violationCount,
	}
	return NewMessage(MessageTypeConstraintUpdate, data)
}

// NewCountsUpdateMessage creates a counts update message for sidebar badge push
func NewCountsUpdateMessage(rgdCount, instanceCount int) (*Message, error) {
	data := CountsUpdateData{
		RGDCount:      rgdCount,
		InstanceCount: instanceCount,
	}
	return NewMessage(MessageTypeCountsUpdate, data)
}

// NewErrorMessage creates an error message
func NewErrorMessage(code, message string) (*Message, error) {
	data := ErrorData{
		Code:    code,
		Message: message,
	}
	return NewMessage(MessageTypeError, data)
}

// Bytes serializes the message to JSON bytes
func (m *Message) Bytes() ([]byte, error) {
	return json.Marshal(m)
}
