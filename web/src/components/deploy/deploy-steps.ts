// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

export interface DeployStep {
  id: string;
  label: string;
  sectionId: string;
}

export const DEPLOY_STEPS: DeployStep[] = [
  { id: "details", label: "Instance Details", sectionId: "deploy-instance-details" },
  { id: "mode", label: "Deployment Mode", sectionId: "deploy-deployment-mode" },
  { id: "config", label: "Configuration", sectionId: "deploy-configuration" },
  { id: "review", label: "Review & Deploy", sectionId: "deploy-review" },
];
