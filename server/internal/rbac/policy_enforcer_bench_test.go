package rbac

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BenchmarkCanAccess_AdminAllow benchmarks admin access check (should be fast cache hit)
func BenchmarkCanAccess_AdminAllow(b *testing.B) {
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		b.Fatal(err)
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * 60 * 1e9, // 5 min in ns
	})

	// Assign admin role
	ctx := context.Background()
	_ = pe.AssignUserRoles(ctx, "admin@test.local", []string{"role:serveradmin"})

	// Warm up cache
	_, _ = pe.CanAccess(ctx, "role:serveradmin", "projects/test", "get")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pe.CanAccess(ctx, "role:serveradmin", "projects/test", "get")
	}
}

// BenchmarkCanAccess_CacheMiss benchmarks enforcement without cache
func BenchmarkCanAccess_CacheMiss(b *testing.B) {
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		b.Fatal(err)
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, PolicyEnforcerConfig{
		CacheEnabled: false,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pe.CanAccess(ctx, "role:serveradmin", "projects/test", "get")
	}
}

// BenchmarkCanAccessWithGroups_SingleGroup benchmarks group-based access check
func BenchmarkCanAccessWithGroups_SingleGroup(b *testing.B) {
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		b.Fatal(err)
	}

	reader := newMockProjectReader()
	reader.AddProject(&Project{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "admin",
					Policies: []string{"*, *, allow"},
					Groups:   []string{"alpha-admins"},
				},
			},
		},
	})

	pe := NewPolicyEnforcerWithConfig(enforcer, reader, PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * 60 * 1e9,
	})

	ctx := context.Background()
	_ = pe.LoadProjectPolicies(ctx, reader.projects["alpha"])

	// Warm up cache
	_, _ = pe.CanAccessWithGroups(ctx, "dev@test.local", []string{"alpha-admins"}, "projects/alpha", "get")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pe.CanAccessWithGroups(ctx, "dev@test.local", []string{"alpha-admins"}, "projects/alpha", "get")
	}
}

// BenchmarkCanAccessWithGroups_MultipleGroups benchmarks with 5 groups
func BenchmarkCanAccessWithGroups_MultipleGroups(b *testing.B) {
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		b.Fatal(err)
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * 60 * 1e9,
	})

	ctx := context.Background()
	groups := []string{"team-a", "team-b", "team-c", "team-d", "team-e"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pe.CanAccessWithGroups(ctx, "user@test.local", groups, "projects/test", "get")
	}
}

// BenchmarkEnforceProjectAccess benchmarks project-specific enforcement
func BenchmarkEnforceProjectAccess(b *testing.B) {
	enforcer, err := NewCasbinEnforcer()
	if err != nil {
		b.Fatal(err)
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * 60 * 1e9,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pe.EnforceProjectAccess(ctx, "role:serveradmin", "test-project", "get")
	}
}

// BenchmarkLoadProjectPolicies benchmarks policy loading for a project with roles and groups
func BenchmarkLoadProjectPolicies(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enforcer, err := NewCasbinEnforcer()
		if err != nil {
			b.Fatal(err)
		}

		pe := NewPolicyEnforcer(enforcer, nil)
		project := &Project{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("project-%d", i)},
			Spec: ProjectSpec{
				Roles: []ProjectRole{
					{
						Name:     "admin",
						Policies: []string{"*, *, allow"},
						Groups:   []string{"admins", "platform-team"},
					},
					{
						Name:     "developer",
						Policies: []string{"instances/*, create, allow", "instances/*, get, allow", "rgds/*, view, allow"},
						Groups:   []string{"developers"},
					},
					{
						Name:     "readonly",
						Policies: []string{"*, view, allow"},
						Groups:   []string{"viewers", "auditors"},
					},
				},
			},
		}
		_ = pe.LoadProjectPolicies(ctx, project)
	}
}

// BenchmarkNormalizeResourceType benchmarks resource type normalization
func BenchmarkNormalizeResourceType(b *testing.B) {
	types := []string{"rgd", "instance", "project", "repository", "*", "unknown"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			_ = normalizeResourceType(t)
		}
	}
}

// BenchmarkNormalizeAction benchmarks action normalization
func BenchmarkNormalizeAction(b *testing.B) {
	actions := []string{"view", "*", "deploy", "create", "update", "delete", "get", "list"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, a := range actions {
			_ = normalizeAction(a)
		}
	}
}

// BenchmarkScopeObjectToProject benchmarks project scoping logic
func BenchmarkScopeObjectToProject(b *testing.B) {
	objects := []string{
		"*",
		"instances/*",
		"repositories/*",
		"projects/*",
		"rgds/*",
		"instances/team-a/my-instance",
		"projects/team-a",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, obj := range objects {
			_ = scopeObjectToProject("test-project", obj)
		}
	}
}
