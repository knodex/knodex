// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCanI } from "@/hooks/useCanI";
import { useCurrentProject } from "@/hooks/useAuth";
import { RepositorySection } from "@/components/repositories/RepositorySection";
import { PageHeader } from "@/components/layout/PageHeader";

export function RepositoriesSettings() {
  const currentProject = useCurrentProject();

  const { allowed: canManageRepos, isLoading: isLoadingRepoPermission, isError: isErrorRepoPermission } = useCanI(
    'repositories',
    'create',
    currentProject || '-'
  );

  return (
    <section className="space-y-6">
      <PageHeader title="Repositories" />
      <RepositorySection canManage={canManageRepos} isLoadingPermission={isLoadingRepoPermission} isErrorPermission={isErrorRepoPermission} />
    </section>
  );
}

export default RepositoriesSettings;
