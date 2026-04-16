// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { InstanceMetadataSection } from './InstanceMetadataSection';
import type { Instance } from '@/types/rgd';

const baseInstance: Instance = {
  name: 'test-instance',
  namespace: 'test-namespace',
  rgdName: 'test-rgd',
  rgdNamespace: 'default',
  apiVersion: 'kro.run/v1alpha1',
  kind: 'TestResource',
  health: 'Healthy',
  conditions: [],
  createdAt: '2024-01-15T10:30:00Z',
  deploymentMode: 'direct',
};

describe('InstanceMetadataSection', () => {
  it('renders "Direct deployment" when not gitops', () => {
    render(<InstanceMetadataSection instance={baseInstance} isGitOps={false} />);

    expect(screen.getByText('Direct deployment')).toBeInTheDocument();
    expect(screen.getByText('Source')).toBeInTheDocument();
  });

  it('renders git branch and commit when gitops', () => {
    const gitopsInstance: Instance = {
      ...baseInstance,
      deploymentMode: 'gitops',
      gitInfo: {
        branch: 'main',
        commitSha: 'abc123def456',
      },
    };

    render(<InstanceMetadataSection instance={gitopsInstance} isGitOps={true} />);

    expect(screen.getByText('abc123de')).toBeInTheDocument();
  });

  it('renders drift indicator when gitopsDrift is true', () => {
    const driftedInstance: Instance = {
      ...baseInstance,
      deploymentMode: 'gitops',
      gitInfo: { branch: 'main' },
      gitopsDrift: true,
    };

    render(<InstanceMetadataSection instance={driftedInstance} isGitOps={true} />);

    expect(screen.getByText('Drifted')).toBeInTheDocument();
  });

  it('renders commit as link when commitUrl is provided', () => {
    const withUrl: Instance = {
      ...baseInstance,
      deploymentMode: 'gitops',
      gitInfo: {
        commitSha: 'abc123def456',
        commitUrl: 'https://github.com/example/commit/abc123',
      },
    };

    render(<InstanceMetadataSection instance={withUrl} isGitOps={true} />);

    const link = screen.getByText('abc123de').closest('a');
    expect(link).toHaveAttribute('href', 'https://github.com/example/commit/abc123');
  });
});
