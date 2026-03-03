package compliance

import "context"

// ComplianceChecker defines the interface for compliance auditing.
// This interface allows handlers to depend on an abstraction rather than
// a concrete implementation, enabling enterprise features through build tags.
//
// In OSS builds, a no-op implementation is used that always passes.
// In Enterprise builds, a full compliance checker is registered via init().
type ComplianceChecker interface {
	// AuditDeployment checks if a deployment meets compliance requirements.
	// Returns an AuditResult containing pass/fail status and any findings.
	AuditDeployment(ctx context.Context, deployment *Deployment) (*AuditResult, error)

	// IsEnabled returns whether compliance checking is active.
	// Returns false in OSS builds, true in Enterprise builds.
	IsEnabled() bool
}
