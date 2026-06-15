// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { render, screen, within, fireEvent } from '@testing-library/react';
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

      // Conditions section present (collapsed since all True)
      expect(screen.getByTestId('conditions-section')).toBeInTheDocument();
      expect(screen.getByText('2/2')).toBeInTheDocument();
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

    it('renders top-level objects as category sections with key-value rows', () => {
      const status: Record<string, unknown> = {
        endpoints: {
          api: 'https://api.example.com',
          admin: 'https://admin.example.com',
        },
      };

      render(<InstanceStatusCard status={status} />);

      // Category section with header (capitalized by formatLabel)
      const group = screen.getByTestId('status-group-endpoints');
      expect(within(group).getByText('Endpoints')).toBeInTheDocument();

      // Both nested keys should be visible as rows inside the section
      expect(within(group).getByText('Api')).toBeInTheDocument();
      expect(within(group).getByText('Admin')).toBeInTheDocument();

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

  describe('Structured status category groups (KRO structured status fields) - Story 55.1', () => {
    it('renders scalars first, then category sections in original key order', () => {
      const status: Record<string, unknown> = {
        state: 'ACTIVE',
        replicas: 3,
        connection: {
          host: '10.96.0.15',
          port: 5432,
        },
        deployment: {
          name: 'my-app',
          namespace: 'demo',
        },
      };

      render(<InstanceStatusCard status={status} />);

      // Flat scalar row, no section for it
      expect(screen.getByText('Replicas')).toBeInTheDocument();
      expect(screen.getByText('3')).toBeInTheDocument();

      // Category sections with headers
      const connection = screen.getByTestId('status-group-connection');
      expect(within(connection).getByText('Connection')).toBeInTheDocument();
      expect(within(connection).getByText('Host')).toBeInTheDocument();
      expect(within(connection).getByText('10.96.0.15')).toBeInTheDocument();
      expect(within(connection).getByText('Port')).toBeInTheDocument();

      const deployment = screen.getByTestId('status-group-deployment');
      expect(within(deployment).getByText('Deployment')).toBeInTheDocument();
      expect(within(deployment).getByText('my-app')).toBeInTheDocument();

      // Scalar rows precede category sections despite original key order
      expect(
        screen.getByText('Replicas').compareDocumentPosition(connection) &
          Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy();

      // Section order follows status key order
      const section = screen.getByTestId('custom-fields-section');
      const groups = within(section).getAllByTestId(/^status-group-/);
      expect(groups.map((g) => g.getAttribute('data-testid'))).toEqual([
        'status-group-connection',
        'status-group-deployment',
      ]);
    });

    it('renders no category headers when status has only flat fields', () => {
      const status: Record<string, unknown> = {
        vnetName: 'vnet-demo',
        location: 'canadacentral',
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('vnet-demo')).toBeInTheDocument();
      expect(screen.queryAllByTestId(/^status-group-/)).toHaveLength(0);
    });

    it('keeps empty objects in the flat list rendered as dash', () => {
      const status: Record<string, unknown> = {
        details: {},
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('Details')).toBeInTheDocument();
      expect(screen.getByText('-')).toBeInTheDocument();
      expect(screen.queryByTestId('status-group-details')).not.toBeInTheDocument();
    });

    it('does not treat arrays as categories', () => {
      const status: Record<string, unknown> = {
        endpoints: ['https://a.example.com', 'https://b.example.com'],
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.queryByTestId('status-group-endpoints')).not.toBeInTheDocument();
      expect(screen.getByText('https://a.example.com')).toBeInTheDocument();
    });

    it('does not promote empty-string keys to sections', () => {
      const status: Record<string, unknown> = {
        '': { replicas: 3 },
      };

      render(<InstanceStatusCard status={status} />);

      // No section (and no empty heading); the object still renders via the flat path
      expect(screen.queryAllByTestId(/^status-group-/)).toHaveLength(0);
      expect(screen.getByText('Replicas')).toBeInTheDocument();
      expect(screen.getByText('3')).toBeInTheDocument();
    });

    it('escapes literal dots in keys so testids cannot collide with nested paths', () => {
      const status: Record<string, unknown> = {
        'a.b': { x: 1 },
        a: { b: { y: 2 } },
      };

      render(<InstanceStatusCard status={status} />);

      // Top-level key "a.b" → escaped testid; nested a→b keeps the plain dot join
      const dotted = screen.getByTestId('status-group-a\\.b');
      expect(within(dotted).getByText('1')).toBeInTheDocument();

      const nested = screen.getByTestId('status-group-a.b');
      expect(within(nested).getByText('2')).toBeInTheDocument();
    });

    it('renders objects nested inside a category as indented sub-sections', () => {
      const status: Record<string, unknown> = {
        deployment: {
          metadata: {
            name: 'my-app',
            namespace: 'demo',
          },
          ready: true,
        },
      };

      render(<InstanceStatusCard status={status} />);

      const group = screen.getByTestId('status-group-deployment');
      expect(within(group).getByText('Deployment')).toBeInTheDocument();
      expect(within(group).getByText('true')).toBeInTheDocument();

      // metadata is a sub-section with dot-path testid, not a value-column sub-list
      const sub = within(group).getByTestId('status-group-deployment.metadata');
      expect(within(sub).getByText('Metadata')).toBeInTheDocument();
      expect(within(sub).getByText('Name')).toBeInTheDocument();
      expect(within(sub).getByText('my-app')).toBeInTheDocument();
      expect(within(sub).getByText('Namespace')).toBeInTheDocument();
      expect(within(sub).getByText('demo')).toBeInTheDocument();
    });
  });

  describe('Nested structured status sub-sections - Story 55.2', () => {
    it('renders the KRO docs nested example as a section with two sub-sections', () => {
      // kro.run/docs/concepts/rgd/schema#structured-status-fields
      const status: Record<string, unknown> = {
        deployment: {
          metadata: {
            name: 'my-app',
            namespace: 'demo',
          },
          status: {
            ready: 2,
            total: 3,
          },
        },
      };

      render(<InstanceStatusCard status={status} />);

      const group = screen.getByTestId('status-group-deployment');
      expect(within(group).getByText('Deployment')).toBeInTheDocument();

      const metadata = within(group).getByTestId('status-group-deployment.metadata');
      expect(within(metadata).getByText('Metadata')).toBeInTheDocument();
      expect(within(metadata).getByText('Name')).toBeInTheDocument();
      expect(within(metadata).getByText('my-app')).toBeInTheDocument();
      expect(within(metadata).getByText('Namespace')).toBeInTheDocument();
      expect(within(metadata).getByText('demo')).toBeInTheDocument();

      const depStatus = within(group).getByTestId('status-group-deployment.status');
      expect(within(depStatus).getByText('Status')).toBeInTheDocument();
      expect(within(depStatus).getByText('Ready')).toBeInTheDocument();
      expect(within(depStatus).getByText('2')).toBeInTheDocument();
      expect(within(depStatus).getByText('Total')).toBeInTheDocument();
      expect(within(depStatus).getByText('3')).toBeInTheDocument();
    });

    it('recurses sub-sections with dot-path testids across multiple levels', () => {
      const status: Record<string, unknown> = {
        network: {
          peering: {
            hub: {
              state: 'Connected',
            },
          },
        },
      };

      render(<InstanceStatusCard status={status} />);

      const top = screen.getByTestId('status-group-network');
      const peering = within(top).getByTestId('status-group-network.peering');
      const hub = within(peering).getByTestId('status-group-network.peering.hub');
      expect(within(hub).getByText('Hub')).toBeInTheDocument();
      expect(within(hub).getByText('State')).toBeInTheDocument();
      expect(within(hub).getByText('Connected')).toBeInTheDocument();
    });

    it('renders scalar members before sub-sections within a section', () => {
      const status: Record<string, unknown> = {
        network: {
          peering: {
            state: 'Connected',
          },
          vnetId: 'vnet-1',
        },
      };

      render(<InstanceStatusCard status={status} />);

      const group = screen.getByTestId('status-group-network');
      const scalarLabel = within(group).getByText('Vnet Id');
      const sub = within(group).getByTestId('status-group-network.peering');

      // scalar row precedes the sub-section despite original key order
      expect(
        scalarLabel.compareDocumentPosition(sub) & Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy();
    });

    it('renders empty object members as dash rows and never treats arrays as sub-sections', () => {
      const status: Record<string, unknown> = {
        network: {
          empty: {},
          ports: [
            { name: 'http', port: 80 },
            { name: 'https', port: 443 },
          ],
          tags: ['a', 'b'],
        },
      };

      render(<InstanceStatusCard status={status} />);

      const group = screen.getByTestId('status-group-network');

      // empty {} stays a "-" row, not an empty sub-section
      expect(within(group).getByText('Empty')).toBeInTheDocument();
      expect(within(group).getByText('-')).toBeInTheDocument();
      expect(within(group).queryByTestId('status-group-network.empty')).not.toBeInTheDocument();

      // arrays keep chips / numbered-list rendering, never sub-sections
      expect(within(group).queryByTestId('status-group-network.ports')).not.toBeInTheDocument();
      expect(within(group).queryByTestId('status-group-network.tags')).not.toBeInTheDocument();
      expect(within(group).getByText('1.')).toBeInTheDocument();
      expect(within(group).getByText('http')).toBeInTheDocument();
      expect(within(group).getByText('a')).toBeInTheDocument();
    });

    it('stops creating sub-sections at the depth cap and falls back to JSON rendering', () => {
      const status: Record<string, unknown> = {
        level1: {
          level2: {
            level3: {
              level4: {
                level5: {
                  level6: {
                    level7: { deep: 'value' },
                  },
                },
              },
            },
          },
        },
      };

      render(<InstanceStatusCard status={status} />);

      // level6 is the deepest sub-section
      expect(
        screen.getByTestId('status-group-level1.level2.level3.level4.level5.level6')
      ).toBeInTheDocument();
      expect(
        screen.queryByTestId('status-group-level1.level2.level3.level4.level5.level6.level7')
      ).not.toBeInTheDocument();

      // the object member beyond the cap renders via the StatusFieldValue depth guard
      expect(screen.getByText('{"deep":"value"}')).toBeInTheDocument();
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
    it('shows conditions collapsed with summary when all True', () => {
      render(<InstanceStatusCard conditions={mockConditions} />);

      const section = screen.getByTestId('conditions-section');
      // Shows count summary
      expect(within(section).getByText('2/2')).toBeInTheDocument();
      // Conditions are collapsed by default when all True
      expect(within(section).queryByText('InstanceSynced')).not.toBeInTheDocument();
    });

    it('expands conditions on click to show details', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      render(<InstanceStatusCard conditions={mockConditions} />);

      const section = screen.getByTestId('conditions-section');
      await user.click(within(section).getByRole('button'));

      expect(within(section).getByText('InstanceSynced')).toBeInTheDocument();
      expect(within(section).getByText('(ReconciliationSucceeded)')).toBeInTheDocument();
      expect(within(section).getByText('All resources are synced')).toBeInTheDocument();
    });

    it('auto-expands conditions when any is False', () => {
      const conditions: InstanceCondition[] = [
        { type: 'Ready', status: 'False', reason: 'NotReady', message: 'Not ready yet' },
      ];

      render(<InstanceStatusCard conditions={conditions} />);

      // Auto-expanded — condition details visible immediately
      expect(screen.getByText('Ready')).toBeInTheDocument();
      expect(screen.getByText('False')).toBeInTheDocument();
    });

    it('renders condition without reason', () => {
      const conditions: InstanceCondition[] = [
        { type: 'Available', status: 'True' },
      ];

      render(<InstanceStatusCard conditions={conditions} />);

      // Click to expand (all True = collapsed)
      fireEvent.click(screen.getByRole('button', { expanded: false }));
      expect(screen.getByText('Available')).toBeInTheDocument();
      expect(screen.queryByText(/\(/)).not.toBeInTheDocument();
    });

    it('renders condition without message', () => {
      const conditions: InstanceCondition[] = [
        { type: 'Synced', status: 'True', reason: 'Synced' },
      ];

      render(<InstanceStatusCard conditions={conditions} />);

      // Click to expand (all True = collapsed)
      fireEvent.click(screen.getByRole('button', { expanded: false }));
      expect(screen.getByText('(Synced)')).toBeInTheDocument();
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

  describe('Outputs treatment (instance-detail redesign)', () => {
    it('titles the card Outputs with a field count and Copy all when custom fields exist', () => {
      const status: Record<string, unknown> = {
        state: 'ACTIVE',
        vnetName: 'vnet-demo',
        location: 'canadacentral',
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByText('Outputs')).toBeInTheDocument();
      expect(screen.getByText('2 fields')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /copy all/i })).toBeInTheDocument();
    });

    it('keeps the Status title when only state is present', () => {
      render(<InstanceStatusCard status={{ state: 'ACTIVE' }} />);

      expect(screen.getByText('Status')).toBeInTheDocument();
      expect(screen.queryByText('Outputs')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /copy all/i })).not.toBeInTheDocument();
    });

    it('renders a per-row copy button for primitive values', () => {
      const status: Record<string, unknown> = {
        vnetName: 'vnet-demo',
        replicas: 3,
        ready: true,
      };

      render(<InstanceStatusCard status={status} />);

      expect(screen.getByRole('button', { name: 'Copy Vnet Name' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Copy Replicas' })).toBeInTheDocument();
      // booleans are not copyable rows
      expect(screen.queryByRole('button', { name: 'Copy Ready' })).not.toBeInTheDocument();
    });

    it('splits resource-ID paths into a dim path and an emphasized leaf', () => {
      const id =
        '/subscriptions/40e663df/resourceGroups/rg-demo-hub/providers/Microsoft.Network/virtualNetworks/vnet-demo-hub';
      render(<InstanceStatusCard status={{ vnetId: id }} />);

      // leaf is its own emphasized segment; full value preserved in title for copy/hover
      expect(screen.getByText('vnet-demo-hub')).toBeInTheDocument();
      expect(screen.getByTitle(id)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Copy Vnet Id' })).toBeInTheDocument();
    });

    it('does not split short or non-path strings', () => {
      render(<InstanceStatusCard status={{ path: '/api/v1', name: 'plain-value' }} />);

      expect(screen.getByText('/api/v1')).toBeInTheDocument();
      expect(screen.getByText('plain-value')).toBeInTheDocument();
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
