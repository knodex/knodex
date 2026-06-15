// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { Breadcrumbs } from './Breadcrumbs';

function renderAt(path: string, ui?: React.ReactNode) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/catalog/categories/:slug" element={ui ?? <Breadcrumbs />} />
        <Route path="/catalog/:rgdName/deploy" element={ui ?? <Breadcrumbs />} />
        <Route path="/catalog/:rgdName" element={ui ?? <Breadcrumbs />} />
        <Route path="/catalog" element={ui ?? <Breadcrumbs />} />
        <Route path="/instances" element={ui ?? <Breadcrumbs />} />
        <Route path="/instances/:group/:version/:namespace/:kind/:name" element={ui ?? <Breadcrumbs />} />
        <Route path="/instances/:group/:version/:kind/:name" element={ui ?? <Breadcrumbs />} />
        <Route path="/secrets" element={ui ?? <Breadcrumbs />} />
        <Route path="/compliance/violations" element={ui ?? <Breadcrumbs />} />
        <Route path="/settings/repositories" element={ui ?? <Breadcrumbs />} />
        <Route path="/settings/license" element={ui ?? <Breadcrumbs />} />
        <Route path="/repositories" element={ui ?? <Breadcrumbs />} />
        <Route path="/projects" element={ui ?? <Breadcrumbs />} />
        <Route path="/projects/:name" element={ui ?? <Breadcrumbs />} />
        <Route path="/user-info" element={ui ?? <Breadcrumbs />} />
        <Route path="/" element={ui ?? <Breadcrumbs />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('Breadcrumbs', () => {
  it('renders Home as default leading crumb when no leadingSlot is given', () => {
    renderAt('/catalog');
    expect(screen.getByText('Home')).toBeInTheDocument();
    expect(screen.getByText('Catalog')).toBeInTheDocument();
  });

  it('omits Home when a leadingSlot is provided', () => {
    renderAt('/catalog', <Breadcrumbs leadingSlot={<span data-testid="chip">proj-a</span>} />);
    expect(screen.queryByText('Home')).not.toBeInTheDocument();
    expect(screen.getByTestId('chip')).toBeInTheDocument();
    expect(screen.getByText('Catalog')).toBeInTheDocument();
  });

  it('omits Home when hideHome is true (e.g., when an org name anchors the root)', () => {
    renderAt('/catalog', <Breadcrumbs hideHome />);
    expect(screen.queryByText('Home')).not.toBeInTheDocument();
    expect(screen.getByText('Catalog')).toBeInTheDocument();
  });

  it('renders the category slug for /catalog/categories/:slug', () => {
    renderAt('/catalog/categories/database', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Catalog')).toBeInTheDocument();
    expect(screen.getByText('database')).toBeInTheDocument();
  });

  it('renders RGD detail crumb for /catalog/:rgdName', () => {
    renderAt('/catalog/AKSCluster', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Catalog')).toBeInTheDocument();
    expect(screen.getByText('AKSCluster')).toBeInTheDocument();
  });

  it('renders Deploy crumb for /catalog/:rgdName/deploy', () => {
    renderAt('/catalog/AKSCluster/deploy', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Catalog')).toBeInTheDocument();
    expect(screen.getByText('AKSCluster')).toBeInTheDocument();
    expect(screen.getByText('Deploy')).toBeInTheDocument();
  });

  it('renders instance crumbs for /instances/:group/:version/:namespace/:kind/:name', () => {
    renderAt('/instances/kro.run/v1alpha1/alpha/MyDB/my-instance', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('MyDB')).toBeInTheDocument();
    expect(screen.getByText('my-instance')).toBeInTheDocument();
  });

  it('renders Settings/Repositories crumb for /settings/repositories', () => {
    renderAt('/settings/repositories', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('Repositories')).toBeInTheDocument();
  });

  it('renders Secrets crumb', () => {
    renderAt('/secrets', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Secrets')).toBeInTheDocument();
  });

  it('marks the last crumb with aria-current=page', () => {
    renderAt('/catalog/categories/database', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    const last = screen.getByText('database');
    expect(last).toHaveAttribute('aria-current', 'page');
  });

  it('renders a "/" separator between crumbs', () => {
    renderAt('/catalog/categories/database', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    // One separator after the leadingSlot + one before "database"
    const separators = screen.getAllByText('/');
    expect(separators.length).toBeGreaterThanOrEqual(2);
  });

  it('uses <ol>/<li> for breadcrumb list semantics', () => {
    renderAt('/catalog', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    const ol = screen.getByTestId('breadcrumbs').querySelector('ol');
    expect(ol).toBeInTheDocument();
    expect(ol?.querySelectorAll('li').length).toBeGreaterThan(0);
  });
});

describe('Breadcrumbs — leaf data-testid (Story 48.12)', () => {
  it('marks the last (non-clickable) crumb with data-testid="topbar-breadcrumb-leaf"', () => {
    renderAt('/catalog', <Breadcrumbs hideHome />);
    const leaf = screen.getByTestId('topbar-breadcrumb-leaf');
    expect(leaf).toHaveTextContent('Catalog');
  });

  it('does not mark intermediate (clickable) crumbs with the leaf testid', () => {
    renderAt('/catalog/categories/database', <Breadcrumbs hideHome />);
    const leaf = screen.getByTestId('topbar-breadcrumb-leaf');
    expect(leaf).toHaveTextContent('database');
    // Catalog should be the clickable link (not the leaf).
    expect(screen.getByRole('link', { name: 'Catalog' })).toBeInTheDocument();
  });
});

describe('Breadcrumbs — Compliance Violations branch (Story 48.12)', () => {
  it('renders Compliance + Violations crumbs for /compliance/violations', () => {
    renderAt('/compliance/violations', <Breadcrumbs hideHome />);
    expect(screen.getByText('Compliance')).toBeInTheDocument();
    expect(screen.getByTestId('topbar-breadcrumb-leaf')).toHaveTextContent('Violations');
  });
});

describe('Breadcrumbs — added route handlers', () => {
  it('renders Account crumb for /user-info', () => {
    renderAt('/user-info', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Account')).toBeInTheDocument();
  });

  it('renders Repositories crumb for top-level /repositories', () => {
    renderAt('/repositories', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Repositories')).toBeInTheDocument();
  });

  it('renders Projects crumb for top-level /projects', () => {
    renderAt('/projects', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Projects')).toBeInTheDocument();
  });

  it('renders Projects + name for /projects/:name', () => {
    renderAt('/projects/alpha', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Projects')).toBeInTheDocument();
    expect(screen.getByText('alpha')).toBeInTheDocument();
  });

  it('renders Settings/License crumb for /settings/license', () => {
    renderAt('/settings/license', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('License')).toBeInTheDocument();
  });
});

describe('Breadcrumbs — safe URL decoding', () => {
  it('does not crash when the URL contains malformed percent-encoding', () => {
    // %E0%A4%A is intentionally broken — decodeURIComponent throws URIError.
    // The breadcrumb must render the raw segment rather than killing the chrome.
    expect(() => {
      renderAt('/projects/%E0%A4%A', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    }).not.toThrow();

    expect(screen.getByText('Projects')).toBeInTheDocument();
    expect(screen.getByText('%E0%A4%A')).toBeInTheDocument();
  });

  it('still decodes well-formed percent-encoded segments', () => {
    renderAt('/projects/proj%20alpha', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('proj alpha')).toBeInTheDocument();
  });
});

describe('Breadcrumbs — instance crumb format', () => {
  it('renders namespace and kind chips plus the name leaf for namespace-scoped instances', () => {
    renderAt(
      '/instances/kro.run/v1alpha1/alpha/MyDB/my-instance',
      <Breadcrumbs leadingSlot={<span>chip</span>} />,
    );
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('MyDB')).toBeInTheDocument();
    // the instance name is the leaf crumb
    expect(screen.getByTestId('topbar-breadcrumb-leaf')).toHaveTextContent('my-instance');
  });

  it('omits the namespace chip for cluster-scoped instances', () => {
    renderAt(
      '/instances/kro.run/v1alpha1/Pod/pod-1',
      <Breadcrumbs leadingSlot={<span>chip</span>} />,
    );
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('Pod')).toBeInTheDocument();
    expect(screen.getByTestId('topbar-breadcrumb-leaf')).toHaveTextContent('pod-1');
  });

  it('decodes percent-encoded instance segments', () => {
    renderAt(
      '/instances/kro.run/v1alpha1/team%20a/MyDB/my%20instance',
      <Breadcrumbs leadingSlot={<span>chip</span>} />,
    );
    expect(screen.getByText('team a')).toBeInTheDocument();
    expect(screen.getByTestId('topbar-breadcrumb-leaf')).toHaveTextContent('my instance');
  });
});
