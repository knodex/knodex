// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { MobileInstanceCard } from './MobileInstanceCard';
import type { Instance } from '@/types/rgd';

// Mock sonner toast
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

// Save and restore navigator.clipboard
const originalClipboard = navigator.clipboard;

const baseInstance: Instance = {
  name: 'test-instance',
  namespace: 'default',
  kind: 'WebApp',
  health: 'Healthy',
  rgdName: 'webapp-rgd',
  createdAt: new Date().toISOString(),
  labels: { 'knodex.io/project': 'alpha' },
  annotations: {},
  isClusterScoped: false,
  status: {},
};

function renderCard(instance: Partial<Instance> = {}, onClick?: (i: Instance) => void) {
  return render(
    <MemoryRouter>
      <MobileInstanceCard
        instance={{ ...baseInstance, ...instance }}
        onClick={onClick}
      />
    </MemoryRouter>
  );
}

describe('MobileInstanceCard', () => {
  afterEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      writable: true,
      configurable: true,
    });
  });

  it('renders instance name and kind', () => {
    renderCard();

    expect(screen.getByText('test-instance')).toBeInTheDocument();
    expect(screen.getByText('WebApp')).toBeInTheDocument();
  });

  it('renders project label', () => {
    renderCard();

    expect(screen.getByText('alpha')).toBeInTheDocument();
  });

  it('renders StatusIndicator', () => {
    renderCard();

    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('does not render copy button when no service URL', () => {
    renderCard();

    expect(screen.queryByTestId('copy-url-button')).not.toBeInTheDocument();
  });

  it('renders copy button when service URL is present', () => {
    renderCard({
      annotations: { 'knodex.io/url': 'https://example.com' },
    });

    expect(screen.getByTestId('copy-url-button')).toBeInTheDocument();
  });

  it('copies URL to clipboard when copy button is clicked', async () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      writable: true,
      configurable: true,
    });

    renderCard({
      annotations: { 'knodex.io/url': 'https://example.com' },
    });

    fireEvent.click(screen.getByTestId('copy-url-button'));
    expect(writeTextMock).toHaveBeenCalledWith('https://example.com');
  });

  it('calls onClick when card is clicked', () => {
    const handleClick = vi.fn();
    renderCard({}, handleClick);

    fireEvent.click(screen.getByTestId('mobile-instance-card'));
    expect(handleClick).toHaveBeenCalledWith(expect.objectContaining({ name: 'test-instance' }));
  });

  it('navigates on Enter key press', () => {
    const handleClick = vi.fn();
    renderCard({}, handleClick);

    fireEvent.keyDown(screen.getByTestId('mobile-instance-card'), { key: 'Enter' });
    expect(handleClick).toHaveBeenCalled();
  });
});
