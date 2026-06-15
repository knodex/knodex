// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useParams, Navigate } from "react-router-dom";
import { DeployPage } from "@/components/deploy/DeployPage";
import RGDDetailRoute from "@/routes/RGDDetailRoute";

export default function DeployRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const decoded = decodeURIComponent(rgdName || "");
  if (!decoded) return <Navigate to="/catalog" replace />;
  // Render the RGD detail page UNDERNEATH the Deploy drawer so the Sheet's
  // dark overlay dims the catalog context the prototype assumes — not an
  // empty black <main>. Without this, opening /deploy/:rgdName as a sibling
  // route (or hitting the URL directly) blanks the area to the left of the
  // drawer. Both views share the same `useRGD` query so React Query dedupes
  // the network calls.
  return (
    <>
      <RGDDetailRoute />
      <DeployPage rgdName={decoded} />
    </>
  );
}
