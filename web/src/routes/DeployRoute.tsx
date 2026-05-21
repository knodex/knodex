// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useParams, Navigate } from "react-router-dom";
import { DeployPage } from "@/components/deploy/DeployPage";

export default function DeployRoute() {
  const { rgdName } = useParams<{ rgdName: string }>();
  const decoded = decodeURIComponent(rgdName || "");
  if (!decoded) return <Navigate to="/catalog" replace />;
  return <DeployPage rgdName={decoded} />;
}
