// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { InstanceConditionsCard } from './InstanceConditionsCard';
import type { InstanceCondition } from '@/types/rgd';

const passingConditions: InstanceCondition[] = [
  { type: 'Ready', status: 'True', message: 'instance reconciled' },
  { type: 'InstanceManaged', status: 'True', message: 'finalizers and labels in place' },
  { type: 'GraphResolved', status: 'True', message: 'runtime graph resolved' },
  { type: 'ResourcesReady', status: 'True', message: 'all resources created' },
];

describe('InstanceConditionsCard', () => {
  it('renders every condition with its message', () => {
    render(<InstanceConditionsCard conditions={passingConditions} />);

    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.getByText('instance reconciled')).toBeInTheDocument();
    expect(screen.getByText('InstanceManaged')).toBeInTheDocument();
    expect(screen.getByText('GraphResolved')).toBeInTheDocument();
    expect(screen.getByText('ResourcesReady')).toBeInTheDocument();
  });

  it('shows "all passing" when every condition is True', () => {
    render(<InstanceConditionsCard conditions={passingConditions} />);

    expect(screen.getByText('all passing')).toBeInTheDocument();
  });

  it('shows passing ratio when a condition is failing', () => {
    const mixed: InstanceCondition[] = [
      ...passingConditions.slice(0, 3),
      { type: 'ResourcesReady', status: 'False', message: '0/2 replicas reconciled' },
    ];
    render(<InstanceConditionsCard conditions={mixed} />);

    expect(screen.getByText('3/4 passing')).toBeInTheDocument();
    expect(screen.getByText('0/2 replicas reconciled')).toBeInTheDocument();
  });

  it('falls back to the reason when no message is present', () => {
    render(
      <InstanceConditionsCard
        conditions={[{ type: 'Ready', status: 'Unknown', reason: 'Reconciling' }]}
      />
    );

    expect(screen.getByText('Reconciling')).toBeInTheDocument();
  });
});
