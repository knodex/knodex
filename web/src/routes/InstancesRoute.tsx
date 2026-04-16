// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { InstancesPage } from "@/components/instances";
import type { Instance } from "@/types/rgd";

export default function InstancesRoute() {
  const navigate = useNavigate();

  const handleInstanceClick = useCallback((instance: Instance) => {
    navigate(`/instances/${encodeURIComponent(instance.namespace)}/${encodeURIComponent(instance.kind)}/${encodeURIComponent(instance.name)}`);
  }, [navigate]);

  return <InstancesPage onInstanceClick={handleInstanceClick} />;
}
