// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package roletemplates provides a ConfigMap-backed CRUD store for the reusable
// catalog of PROJECT-role templates (e.g. developer, operator, readonly) that
// the web UI seeds from when an operator creates a project or adds a role
// (Story 18.1).
//
// A template is ONLY a UI seed: it is copied verbatim into Project.spec.roles[]
// at project create/edit time. Editing or deleting a template does NOT
// retroactively change roles already embedded in existing projects, and there
// is no reconcile/sync loop. Templates never participate in Casbin Enforce() —
// the single enforcement layer (NFR-T1) is unchanged. The stored policy strings
// keep their {project}/{role} placeholders; placeholder resolution and the
// enterprise-only compliance-policy injection happen client-side at apply time.
//
// Persistence is a single ConfigMap (knodex-role-templates) in the install
// namespace, mirroring the category-sidebar-ordering ConfigMap precedent — NOT
// Postgres (reserved for identity/audit/compliance/license) and NOT Redis
// (transient cache). Unlike categories (read-only, cached once at startup) this
// is mutable CRUD, so every method does a live Get/Update/Create per request.
package roletemplates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMapName is the name of the ConfigMap that stores the role-template
// catalog in the install namespace.
const ConfigMapName = "knodex-role-templates"

// dataKey is the ConfigMap data key holding the JSON-encoded template slice.
const dataKey = "templates"

// maxNameLength matches the CRD role-name pattern (DNS-1123 label, ≤63 chars).
const maxNameLength = 63

// nameRe matches a DNS-1123-style role-template name.
var nameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Sentinel errors so the HTTP handler can map store outcomes to status codes
// without importing k8s error helpers (mirrors team_handler's IsNotFound /
// IsAlreadyExists branching, but over store-owned errors).
var (
	// ErrNotFound is returned when a named template does not exist.
	ErrNotFound = errors.New("role template not found")
	// ErrAlreadyExists is returned by Create when the name is already taken.
	ErrAlreadyExists = errors.New("role template already exists")
	// ErrConflict is returned by persist when a concurrent write modified the
	// ConfigMap between load and persist. The caller should retry the operation.
	ErrConflict = errors.New("role template store conflict: concurrent modification, please retry")
)

// ValidationError wraps an invalid-input failure with the offending field so
// the handler can surface a 400 with a field map.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// RoleTemplate is a reusable project-role preset. Policies keep their
// {project} and {role} placeholders (e.g.
// "p, proj:{project}:{role}, instances, *, */{project}/*, allow"); they are
// resolved client-side when the template is applied to a project.
type RoleTemplate struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Policies    []string `json:"policies"`
}

// Store is the ConfigMap-backed CRUD store for role templates.
type Store struct {
	client    kubernetes.Interface
	namespace string
}

// NewStore creates a Store bound to the given clientset and install namespace.
func NewStore(client kubernetes.Interface, namespace string) *Store {
	return &Store{client: client, namespace: namespace}
}

// List returns all templates. When the backing ConfigMap is absent or carries
// no templates, the three built-in defaults are returned as data (NOT written)
// so the out-of-the-box UX is preserved with zero config. The first mutating
// call (Create/Update/Delete) materializes the ConfigMap.
func (s *Store) List(ctx context.Context) ([]RoleTemplate, error) {
	templates, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return templates, nil
}

// Get returns a single template by name, or ErrNotFound.
func (s *Store) Get(ctx context.Context, name string) (*RoleTemplate, error) {
	templates, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range templates {
		if templates[i].Name == name {
			t := templates[i]
			return &t, nil
		}
	}
	return nil, ErrNotFound
}

// Create validates and appends a new template. Returns ErrAlreadyExists if the
// name is taken, or a *ValidationError for invalid input.
func (s *Store) Create(ctx context.Context, t RoleTemplate) (*RoleTemplate, error) {
	if err := validate(t); err != nil {
		return nil, err
	}
	templates, cm, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	for i := range templates {
		if templates[i].Name == t.Name {
			return nil, ErrAlreadyExists
		}
	}
	templates = append(templates, t)
	if err := s.persist(ctx, templates, cm); err != nil {
		return nil, err
	}
	created := t
	return &created, nil
}

// Update validates and replaces an existing template (matched by path name).
// The name in the body is ignored; the path name is authoritative and
// immutable. Returns ErrNotFound if absent.
func (s *Store) Update(ctx context.Context, name string, t RoleTemplate) (*RoleTemplate, error) {
	t.Name = name
	if err := validate(t); err != nil {
		return nil, err
	}
	templates, cm, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for i := range templates {
		if templates[i].Name == name {
			templates[i] = t
			found = true
			break
		}
	}
	if !found {
		return nil, ErrNotFound
	}
	if err := s.persist(ctx, templates, cm); err != nil {
		return nil, err
	}
	updated := t
	return &updated, nil
}

// Delete removes a template by name. Returns ErrNotFound if absent.
func (s *Store) Delete(ctx context.Context, name string) error {
	templates, cm, err := s.load(ctx)
	if err != nil {
		return err
	}
	idx := -1
	for i := range templates {
		if templates[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	templates = append(templates[:idx], templates[idx+1:]...)
	return s.persist(ctx, templates, cm)
}

// load fetches the backing ConfigMap and returns the current template set plus
// the ConfigMap (nil when absent). When the ConfigMap is missing or has no
// templates key, the built-in defaults are returned so callers see a populated
// catalog; a nil ConfigMap signals persist() to Create rather than Update.
func (s *Store) load(ctx context.Context) ([]RoleTemplate, *corev1.ConfigMap, error) {
	cm, err := s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return DefaultRoleTemplates(), nil, nil
		}
		return nil, nil, fmt.Errorf("get role-templates ConfigMap: %w", err)
	}
	raw, ok := cm.Data[dataKey]
	if !ok || raw == "" {
		return DefaultRoleTemplates(), cm, nil
	}
	var templates []RoleTemplate
	if err := json.Unmarshal([]byte(raw), &templates); err != nil {
		return nil, nil, fmt.Errorf("decode role-templates ConfigMap: %w", err)
	}
	if templates == nil {
		templates = []RoleTemplate{}
	}
	return templates, cm, nil
}

// persist writes the template set back to the ConfigMap. When cm is nil the
// ConfigMap is Created (first materialization); otherwise it is Updated in
// place, preserving metadata for optimistic concurrency.
//
// Concurrent-write handling:
//   - Create path (cm==nil): if a racing request already created the CM, fall
//     through to the Update path with the freshly fetched CM.
//   - Update path: if the CM was modified between load and persist (stale
//     resourceVersion), return ErrConflict so the handler returns 409 and the
//     client can retry.
func (s *Store) persist(ctx context.Context, templates []RoleTemplate, cm *corev1.ConfigMap) error {
	encoded, err := json.Marshal(templates)
	if err != nil {
		return fmt.Errorf("encode role-templates: %w", err)
	}
	data := string(encoded)
	if cm == nil {
		newCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: s.namespace,
			},
			Data: map[string]string{dataKey: data},
		}
		_, createErr := s.client.CoreV1().ConfigMaps(s.namespace).Create(ctx, newCM, metav1.CreateOptions{})
		if createErr == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(createErr) {
			return fmt.Errorf("create role-templates ConfigMap: %w", createErr)
		}
		// A concurrent first-write beat us: reload the existing CM so we can
		// fall through to the Update path below with the real ResourceVersion.
		cm, err = s.client.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("reload role-templates ConfigMap after create conflict: %w", err)
		}
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[dataKey] = data
	if _, err := s.client.CoreV1().ConfigMaps(s.namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return ErrConflict
		}
		return fmt.Errorf("update role-templates ConfigMap: %w", err)
	}
	return nil
}

// validate enforces the RoleTemplate shape: DNS-1123 name (≤63 chars) and at
// least one policy string. Returns a *ValidationError on failure.
func validate(t RoleTemplate) error {
	if t.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if len(t.Name) > maxNameLength {
		return &ValidationError{Field: "name", Message: fmt.Sprintf("name must be %d characters or fewer", maxNameLength)}
	}
	if !nameRe.MatchString(t.Name) {
		return &ValidationError{Field: "name", Message: "name must be lowercase alphanumeric with hyphens (DNS-1123 label)"}
	}
	if len(t.Policies) == 0 {
		return &ValidationError{Field: "policies", Message: "at least one policy is required"}
	}
	for i, p := range t.Policies {
		if strings.TrimSpace(p) == "" {
			return &ValidationError{Field: "policies", Message: fmt.Sprintf("policy at index %d is empty or whitespace", i)}
		}
	}
	return nil
}
