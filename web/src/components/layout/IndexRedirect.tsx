// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Navigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useUserStore } from "@/stores/userStore";
import { getAccountInfo } from "@/api/auth";
import { STALE_TIME } from "@/lib/query-client";
import { PageSkeleton } from "@/components/ui/page-skeleton";

/**
 * IndexRedirect is the landing-redirect seam for the protected index route
 * (story 17.1). Historically `/` redirected unconditionally to `/instances`,
 * which 403-walls a `member` with no project bindings into a broken-empty shell.
 *
 * This gate resolves the caller's server-authoritative account info and sends a
 * bare member — `applicationRole === 'member'` AND no bound projects — to the
 * self-scoped "My Access" view (`/user-info`) instead. Everyone else (serveradmin
 * or any bound user) keeps the original `/instances` landing.
 *
 * It renders the page loader until account info resolves so the redirect does not
 * flash. `DashboardLayout` already gates the whole tree on a valid session, so this
 * only runs for authenticated users; if the fetch fails we fall back to the
 * historical `/instances` behavior rather than trapping the user on a loader.
 */
export function IndexRedirect() {
  const user = useUserStore((s) => s.user);

  const { data: accountInfo, isLoading, isError } = useQuery({
    queryKey: ["account", "info"],
    queryFn: getAccountInfo,
    enabled: !!user,
    staleTime: STALE_TIME.STANDARD,
  });

  // Keep showing the loader while account info is in flight so we never flash a
  // redirect to the wrong landing. Only wait while a fetch is actually possible.
  if (user && isLoading && !isError) {
    return <PageSkeleton />;
  }

  const isBareMember =
    accountInfo?.applicationRole === "member" &&
    (accountInfo?.projects?.length ?? 0) === 0;

  return <Navigate to={isBareMember ? "/user-info" : "/instances"} replace />;
}

export default IndexRedirect;
