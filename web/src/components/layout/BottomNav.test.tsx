// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { BottomNav } from './BottomNav';

function renderBottomNav(initialPath = '/instances') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <BottomNav />
    </MemoryRouter>
  );
}

describe('BottomNav', () => {
  it('renders 4 navigation items', () => {
    renderBottomNav();

    expect(screen.getByText('Instances')).toBeInTheDocument();
    expect(screen.getByText('Catalog')).toBeInTheDocument();
    expect(screen.getByText('Projects')).toBeInTheDocument();
    expect(screen.getByText('Account')).toBeInTheDocument();
  });

  it('has correct navigation labels as aria-labels', () => {
    renderBottomNav();

    expect(screen.getByRole('link', { name: 'Instances' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Catalog' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Projects' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Account' })).toBeInTheDocument();
  });

  it('marks Instances as active on /instances path', () => {
    renderBottomNav('/instances');

    const instancesLink = screen.getByRole('link', { name: 'Instances' });
    expect(instancesLink).toHaveAttribute('aria-current', 'page');
  });

  it('marks Catalog as active on /catalog path', () => {
    renderBottomNav('/catalog');

    const catalogLink = screen.getByRole('link', { name: 'Catalog' });
    expect(catalogLink).toHaveAttribute('aria-current', 'page');

    const instancesLink = screen.getByRole('link', { name: 'Instances' });
    expect(instancesLink).not.toHaveAttribute('aria-current');
  });

  it('marks Projects as active on /projects path', () => {
    renderBottomNav('/projects');

    const projectsLink = screen.getByRole('link', { name: 'Projects' });
    expect(projectsLink).toHaveAttribute('aria-current', 'page');
  });

  it('marks Account as active on /user-info path', () => {
    renderBottomNav('/user-info');

    const accountLink = screen.getByRole('link', { name: 'Account' });
    expect(accountLink).toHaveAttribute('aria-current', 'page');
  });

  it('has correct link destinations', () => {
    renderBottomNav();

    expect(screen.getByRole('link', { name: 'Instances' })).toHaveAttribute('href', '/instances');
    expect(screen.getByRole('link', { name: 'Catalog' })).toHaveAttribute('href', '/catalog');
    expect(screen.getByRole('link', { name: 'Projects' })).toHaveAttribute('href', '/projects');
    expect(screen.getByRole('link', { name: 'Account' })).toHaveAttribute('href', '/user-info');
  });

  it('renders a nav element with mobile navigation label', () => {
    renderBottomNav();

    const nav = screen.getByRole('navigation', { name: 'Mobile navigation' });
    expect(nav).toBeInTheDocument();
  });

  it('renders exactly 4 links', () => {
    renderBottomNav();

    const links = screen.getAllByRole('link');
    expect(links).toHaveLength(4);
  });
});
