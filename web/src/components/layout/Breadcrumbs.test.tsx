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
        <Route path="/instances/group/:group/ns/:namespace/:kind/:name" element={ui ?? <Breadcrumbs />} />
        <Route path="/instances/group/:group/cluster/:kind/:name" element={ui ?? <Breadcrumbs />} />
        <Route path="/instances/:namespace/:kind/:name" element={ui ?? <Breadcrumbs />} />
        <Route path="/secrets" element={ui ?? <Breadcrumbs />} />
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

  it('renders instance crumb for /instances/:namespace/:kind/:name', () => {
    renderAt('/instances/alpha/MyDB/my-instance', <Breadcrumbs leadingSlot={<span>chip</span>} />);
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('alpha/MyDB/my-instance')).toBeInTheDocument();
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

describe('Breadcrumbs — instance label format', () => {
  it('renders namespace/kind/name for namespace-scoped instances', () => {
    renderAt(
      '/instances/group/myGroup/ns/alpha/MyDB/my-instance',
      <Breadcrumbs leadingSlot={<span>chip</span>} />,
    );
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('alpha/MyDB/my-instance')).toBeInTheDocument();
  });

  it('renders kind/name without leading slash for cluster-scoped instances', () => {
    renderAt(
      '/instances/group/myGroup/cluster/Pod/pod-1',
      <Breadcrumbs leadingSlot={<span>chip</span>} />,
    );
    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('Pod/pod-1')).toBeInTheDocument();
    // Regression guard: no leading slash on cluster-scoped label
    expect(screen.queryByText('/Pod/pod-1')).not.toBeInTheDocument();
  });
});
