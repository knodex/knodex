// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { InstanceStatusCard } from './InstanceStatusCard';
import type { InstanceCondition } from '@/types/rgd';

const mockConditions: InstanceCondition[] = [
  {
    type: 'InstanceSynced',
    status: 'True',
    reason: 'ReconciliationSucceeded',
    message: 'All resources are synced',
  },
  {
    type: 'Ready',
    status: 'True',
    reason: 'MinimumReplicasAvailable',
    message: 'Deployment has minimum replicas',
  },
];

describe('InstanceStatusCard', () => {
  describe('Full status (state + custom fields + conditions) - AC-1', () => {
    it('renders unified status card with all sections', () => {
      const status: Record<string, unknown> = {
        state: 'ACTIVE',
        conditions: [],
        serviceIP: '10.96.0.15',
        availableReplicas: 3,
      };

      render(<InstanceStatusCard status={status} conditions={mockConditions} />);

      const card = screen.getByTestId('instance-status-card');
      expect(card).toBeInTheDocument();

      // State badge in header
      expect(screen.getByTestId('state-badge')).toHaveTextContent('ACTIVE');

      // Custom fields section
      expect(screen.getByTestId('custom-fields-section')).toBeInTheDocument();
      expect(screen.getByText('10.96.0.15')).toBeInTheDocument();
      expect(screen.getByText('3')).toBeInTheDocument();

      // Conditions section
      expect(screen.getByTestId('conditions-section')).toBeInTheDocument();
      expect(screen.getByText('InstanceSynced')).toBeInTheDocument();
      expect(screen.getByText('Ready')).toBeInTheDocument();
    });
  });

  describe('Only conditions (no custom fields) - AC-8', () => {
    it('renders card with state + conditions only', () => {
      const status: Record<string, unknown> = {
        state: 'ACTIVE',
        conditions: [],
      };

      render(<InstanceStatusCard status={status} conditions={mockConditions} />);

      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
      expect(screen.getByTestId('state-badge')).toHaveTextContent('ACTIVE');
      expect(screen.queryByTestId('custom-fields-section')).not.toBeInTheDocument();
      expect(screen.getByTestId('conditions-section')).toBeInTheDocument();
    });

    it('renders conditions without status object', () => {
      render(<InstanceStatusCard conditions={mockConditions} />);

      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
      expect(screen.queryByTestId('state-badge')).not.toBeInTheDocument();
      expect(screen.getByTestId('conditions-section')).toBeInTheDocument();
    });
  });

  describe('Only custom fields (no conditions) - AC-8', () => {
    it('renders card with state + custom fields only', () => {
      const status: Record<string, unknown> = {
        state: 'IN_PROGRESS',
        serviceIP: '10.0.0.1',
      };

      render(<InstanceStatusCard status={status} conditions={[]} />);

      expect(screen.getByTestId('instance-status-card')).toBeInTheDocument();
      expect(screen.getByTestId('state-badge')).toHaveTextContent('IN_PROGRESS');
      expect(screen.getByTestId('custom-fields-section')).toBeInTheDocument();
      expect(screen.queryByTestId('conditions-section')).not.toBeInTheDocument();
    });
  });

  describe('Value types (scalars, nested objects, arrays, booleans) - AC-4, AC-5, AC-6', () => {
    it('renders string values as text', () => {
      const status: Record<string, unknown> = {
        connectionString: 'mongodb://localhost:27017',
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('mongodb://localhost:27017')).toBeInTheDocument();
    });

    it('renders number values as text', () => {
      const status: Record<string, unknown> = {
        availableReplicas: 5,
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('5')).toBeInTheDocument();
    });

    it('renders boolean true with check icon', () => {
      const status: Record<string, unknown> = {
        isReady: true,
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('true')).toBeInTheDocument();
    });

    it('renders boolean false with X icon', () => {
      const status: Record<string, unknown> = {
        isReady: false,
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('false')).toBeInTheDocument();
    });

    it('renders null values as dash', () => {
      const status: Record<string, unknown> = {
        optionalField: null,
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('-')).toBeInTheDocument();
    });

    it('renders URLs as clickable links', () => {
      const status: Record<string, unknown> = {
        endpoint: 'https://api.example.com',
      };

      render(<InstanceStatusCard status={status} />);

      const link = screen.getByRole('link', { name: /api\.example\.com/i });
      expect(link).toHaveAttribute('href', 'https://api.example.com');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('renders nested objects with indented key-value pairs', () => {
      const status: Record<string, unknown> = {
        endpoints: {
          api: 'https://api.example.com',
          admin: 'https://admin.example.com',
        },
      };

      render(<InstanceStatusCard status={status} />);

      // Both nested keys should be visible (capitalized by formatLabel)
      expect(screen.getByText('Api')).toBeInTheDocument();
      expect(screen.getByText('Admin')).toBeInTheDocument();

      // Values should be rendered as links
      expect(screen.getByRole('link', { name: /api\.example\.com/i })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: /admin\.example\.com/i })).toBeInTheDocument();
    });

    it('renders arrays of primitives as chips', () => {
      const status: Record<string, unknown> = {
        readyNodes: ['node-1', 'node-2', 'node-3'],
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('node-1')).toBeInTheDocument();
      expect(screen.getByText('node-2')).toBeInTheDocument();
      expect(screen.getByText('node-3')).toBeInTheDocument();
    });

    it('renders empty arrays as dash', () => {
      const status: Record<string, unknown> = {
        tags: [],
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('-')).toBeInTheDocument();
    });

    it('renders arrays of objects as numbered list', () => {
      const status: Record<string, unknown> = {
        ports: [
          { name: 'http', port: 80 },
          { name: 'https', port: 443 },
        ],
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('1.')).toBeInTheDocument();
      expect(screen.getByText('2.')).toBeInTheDocument();
      expect(screen.getByText('http')).toBeInTheDocument();
      expect(screen.getByText('https')).toBeInTheDocument();
    });
  });

  describe('State badge with all KRO state values - AC-2', () => {
    const stateTests: [string, RegExp][] = [
      ['ACTIVE', /ACTIVE/],
      ['IN_PROGRESS', /IN_PROGRESS/],
      ['FAILED', /FAILED/],
      ['DELETING', /DELETING/],
      ['ERROR', /ERROR/],
    ];

    it.each(stateTests)('renders %s state badge', (state, pattern) => {
      const status: Record<string, unknown> = { state };
      render(<InstanceStatusCard status={status} />);

      const badge = screen.getByTestId('state-badge');
      expect(badge).toHaveTextContent(pattern);
    });

    it('renders badge with unknown state value', () => {
      const status: Record<string, unknown> = { state: 'CUSTOM_STATE' };
      render(<InstanceStatusCard status={status} />);

      const badge = screen.getByTestId('state-badge');
      expect(badge).toHaveTextContent('CUSTOM_STATE');
    });

    it('does not render badge when state is undefined', () => {
      const status: Record<string, unknown> = { serviceIP: '10.0.0.1' };
      render(<InstanceStatusCard status={status} />);

      expect(screen.queryByTestId('state-badge')).not.toBeInTheDocument();
    });
  });

  describe('Empty status renders nothing - AC-8', () => {
    it('renders nothing when status is undefined and no conditions', () => {
      const { container } = render(<InstanceStatusCard />);
      expect(container.firstChild).toBeNull();
    });

    it('renders nothing when status is empty object and no conditions', () => {
      const { container } = render(<InstanceStatusCard status={{}} conditions={[]} />);
      expect(container.firstChild).toBeNull();
    });

    it('renders nothing when status has only state/conditions keys but empty conditions', () => {
      const status: Record<string, unknown> = { conditions: [] };
      const { container } = render(<InstanceStatusCard status={status} conditions={[]} />);
      expect(container.firstChild).toBeNull();
    });
  });

  describe('Conditions rendering preserved - AC-7', () => {
    it('renders condition with colored dot, type, reason, message, and status badge', () => {
      render(<InstanceStatusCard conditions={mockConditions} />);

      const section = screen.getByTestId('conditions-section');

      // Type
      expect(within(section).getByText('InstanceSynced')).toBeInTheDocument();
      // Reason in parens
      expect(within(section).getByText('(ReconciliationSucceeded)')).toBeInTheDocument();
      // Message
      expect(within(section).getByText('All resources are synced')).toBeInTheDocument();
      // Status badge
      const trueBadges = within(section).getAllByText('True');
      expect(trueBadges.length).toBeGreaterThanOrEqual(1);
    });

    it('renders condition with False status in destructive color', () => {
      const conditions: InstanceCondition[] = [
        { type: 'Ready', status: 'False', reason: 'NotReady', message: 'Not ready yet' },
      ];

      render(<InstanceStatusCard conditions={conditions} />);

      const statusBadge = screen.getByText('False');
      expect(statusBadge).toBeInTheDocument();
    });

    it('renders condition without reason', () => {
      const conditions: InstanceCondition[] = [
        { type: 'Available', status: 'True' },
      ];

      render(<InstanceStatusCard conditions={conditions} />);

      expect(screen.getByText('Available')).toBeInTheDocument();
      expect(screen.queryByText(/\(/)).not.toBeInTheDocument();
    });

    it('renders condition without message', () => {
      const conditions: InstanceCondition[] = [
        { type: 'Synced', status: 'True', reason: 'Synced' },
      ];

      render(<InstanceStatusCard conditions={conditions} />);

      expect(screen.getByText('Synced')).toBeInTheDocument();
    });

    it('renders Conditions sub-header', () => {
      render(<InstanceStatusCard conditions={mockConditions} />);

      expect(screen.getByText('Conditions')).toBeInTheDocument();
    });
  });

  describe('camelCase label formatting', () => {
    it('converts camelCase keys to capitalized readable labels', () => {
      const status: Record<string, unknown> = {
        availableReplicas: 3,
        serviceIP: '10.0.0.1',
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('Available Replicas')).toBeInTheDocument();
      expect(screen.getByText('Service IP')).toBeInTheDocument();
    });

    it('converts snake_case keys to capitalized readable labels', () => {
      const status: Record<string, unknown> = {
        ready_replicas: 2,
        pod_cidr: '10.244.0.0/16',
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('Ready replicas')).toBeInTheDocument();
      expect(screen.getByText('Pod cidr')).toBeInTheDocument();
    });
  });

  describe('Depth guard for deeply nested objects - robustness', () => {
    it('renders JSON fallback when nesting exceeds depth limit', () => {
      const status: Record<string, unknown> = {
        level1: {
          level2: {
            level3: {
              level4: {
                level5: {
                  level6: {
                    deep: 'value',
                  },
                },
              },
            },
          },
        },
      };

      render(<InstanceStatusCard status={status} />);

      // At depth 6, the string value is rendered via JSON.stringify (with quotes)
      expect(screen.getByText('"value"')).toBeInTheDocument();
    });
  });

  describe('Edge case: conditions in status but not as prop', () => {
    it('does not render status.conditions as a custom field', () => {
      const status: Record<string, unknown> = {
        state: 'ACTIVE',
        conditions: [{ type: 'Ready', status: 'True' }],
        serviceIP: '10.0.0.1',
      };

      // Pass conditions in status but NOT as a separate prop
      render(<InstanceStatusCard status={status} />);

      // serviceIP should appear as custom field
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();

      // conditions should NOT appear as a custom field (filtered by getCustomFields)
      // but since no conditions prop, the conditions section should be hidden
      expect(screen.queryByTestId('conditions-section')).not.toBeInTheDocument();
    });
  });
});
