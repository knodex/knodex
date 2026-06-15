// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { DeployRGDPage } from "@/components/deploy/DeployRGDPage";

/**
 * Deploy RGD route (Story 50.2): /deploy-rgd. Carries the generated spec in
 * router state from the RGD Builder's "Use this spec" action; direct
 * navigation without state redirects back to the builder (handled inside
 * DeployRGDPage).
 */
export default function DeployRGDRoute() {
  return <DeployRGDPage />;
}
