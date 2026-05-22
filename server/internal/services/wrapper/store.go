// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// dns1123SubdomainRegex matches DNS-1123 subdomain (RFC 1123 § 2.1).
// Same shape kro enforces on RGD resource names.
var dns1123SubdomainRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// Store persists wrapper registry entries in a Kubernetes ConfigMap.
// CRUD operations always read-modify-write the ConfigMap as a whole; entries
// are keyed by Kind (uniqueness enforced on Put).
type Store struct {
	k8sClient kubernetes.Interface
	namespace string
}

// NewStore returns a wrapper.Store bound to the given namespace.
func NewStore(k8sClient kubernetes.Interface, namespace string) *Store {
	return &Store{k8sClient: k8sClient, namespace: namespace}
}

// ValidateKind verifies that kind is in the supported allowlist.
// Future Kinds widen SupportedKinds() without code changes in callers.
func ValidateKind(kind string) error {
	if kind == "" {
		return fmt.Errorf("kind cannot be empty")
	}
	if !IsSupportedKind(kind) {
		return fmt.Errorf("wrappers for Kind %q are not yet supported in this release", kind)
	}
	return nil
}

// ValidateRGDName verifies that name is a valid DNS-1123 subdomain.
// kro RGD names must satisfy this constraint, so the wrapper-registry refuses
// values that the cluster will reject at apply time.
func ValidateRGDName(name string) error {
	if name == "" {
		return fmt.Errorf("rgdName cannot be empty")
	}
	if len(name) > MaxRGDNameLength {
		return fmt.Errorf("rgdName must be %d characters or less, got %d", MaxRGDNameLength, len(name))
	}
	if !dns1123SubdomainRegex.MatchString(name) {
		return fmt.Errorf("rgdName must be a valid DNS-1123 subdomain")
	}
	return nil
}

// List returns all wrapper entries from the ConfigMap.
// Missing ConfigMap or empty data → empty slice (not an error).
// Malformed JSON → error (callers decide whether to keep last-valid).
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	cm, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("reading wrapper ConfigMap: %w", err)
	}

	data, ok := cm.Data[ConfigMapKey]
	if !ok || data == "" {
		return []Entry{}, nil
	}

	var entries []Entry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		return nil, fmt.Errorf("parsing wrapper entries JSON: %w", err)
	}
	return entries, nil
}

// Get returns the entry for the given Kind, or *NotFoundError if absent.
func (s *Store) Get(ctx context.Context, kind string) (*Entry, error) {
	entries, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Kind == kind {
			entry := e
			return &entry, nil
		}
	}
	return nil, &NotFoundError{Kind: kind}
}

// Put upserts a wrapper entry. Creates the ConfigMap if missing.
// Kind must be in SupportedKinds; RGDName must be a valid DNS-1123 subdomain.
func (s *Store) Put(ctx context.Context, entry Entry) error {
	if err := ValidateKind(entry.Kind); err != nil {
		return fmt.Errorf("invalid wrapper entry: %w", err)
	}
	if err := ValidateRGDName(entry.RGDName); err != nil {
		return fmt.Errorf("invalid wrapper entry: %w", err)
	}

	entries, err := s.List(ctx)
	if err != nil {
		return err
	}

	// Upsert by Kind.
	updated := false
	for i, e := range entries {
		if e.Kind == entry.Kind {
			entries[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, entry)
	}

	return s.writeEntries(ctx, entries)
}

// Delete removes the entry for the given Kind. *NotFoundError when absent.
func (s *Store) Delete(ctx context.Context, kind string) error {
	entries, err := s.List(ctx)
	if err != nil {
		return err
	}

	found := false
	newEntries := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Kind == kind {
			found = true
			continue
		}
		newEntries = append(newEntries, e)
	}
	if !found {
		return &NotFoundError{Kind: kind}
	}

	return s.writeEntries(ctx, newEntries)
}

// writeEntries marshals entries and writes (or creates) the ConfigMap.
func (s *Store) writeEntries(ctx context.Context, entries []Entry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshaling wrapper entries: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: s.namespace,
			Labels: map[string]string{
				LabelManagedBy:  LabelManagedByVal,
				LabelConfigType: LabelConfigTypeVal,
			},
		},
		Data: map[string]string{
			ConfigMapKey: string(data),
		},
	}

	existing, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, createErr := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Create(ctx, cm, metav1.CreateOptions{}); createErr != nil {
				return fmt.Errorf("creating wrapper ConfigMap: %w", createErr)
			}
			return nil
		}
		return fmt.Errorf("reading existing wrapper ConfigMap: %w", err)
	}

	existing.Data = cm.Data
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range cm.Labels {
		existing.Labels[k] = v
	}
	if _, err := s.k8sClient.CoreV1().ConfigMaps(s.namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating wrapper ConfigMap: %w", err)
	}
	return nil
}

// NotFoundError is returned when a wrapper entry for a Kind is not found.
type NotFoundError struct {
	Kind string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("wrapper entry for kind %q not found", e.Kind)
}

// IsNotFound reports whether err is a *NotFoundError.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}
