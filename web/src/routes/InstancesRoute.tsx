// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { InstancesPage } from "@/components/instances";
import { buildInstanceRoute } from "@/lib/instancePath";
import type { Instance } from "@/types/rgd";

export default function InstancesRoute() {
  const navigate = useNavigate();

  const handleInstanceClick = useCallback((instance: Instance) => {
    navigate(buildInstanceRoute(instance));
  }, [navigate]);

  return <InstancesPage onInstanceClick={handleInstanceClick} />;
}
