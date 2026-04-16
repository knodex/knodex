// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { CatalogPage } from "@/components/catalog";
import type { CatalogRGD } from "@/types/rgd";

export default function CatalogRoute() {
  const navigate = useNavigate();

  const handleRGDClick = useCallback((rgd: CatalogRGD) => {
    navigate(`/catalog/${encodeURIComponent(rgd.name)}`);
  }, [navigate]);

  return <CatalogPage onRGDClick={handleRGDClick} />;
}
