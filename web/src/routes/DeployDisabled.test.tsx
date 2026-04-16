// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import DeployDisabled from './DeployDisabled';

const originalClipboard = navigator.clipboard;
const originalShare = navigator.share;

function renderDeployDisabled() {
  return render(
    <MemoryRouter>
      <DeployDisabled />
    </MemoryRouter>
  );
}

describe('DeployDisabled', () => {
  afterEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      writable: true,
      configurable: true,
    });
    Object.defineProperty(navigator, 'share', {
      value: originalShare,
      writable: true,
      configurable: true,
    });
  });

  it('renders the disabled message', () => {
    renderDeployDisabled();

    expect(screen.getByText('Deploy is only available on desktop')).toBeInTheDocument();
  });

  it('renders the share link button', () => {
    renderDeployDisabled();

    expect(screen.getByTestId('share-link-button')).toBeInTheDocument();
    expect(screen.getByText('Share this link')).toBeInTheDocument();
  });

  it('renders the back to instances link', () => {
    renderDeployDisabled();

    const backLink = screen.getByTestId('back-to-instances');
    expect(backLink).toBeInTheDocument();
    expect(backLink).toHaveAttribute('href', '/instances');
  });

  it('copies URL to clipboard when share is clicked and navigator.share is unavailable', () => {
    const writeTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      writable: true,
      configurable: true,
    });
    Object.defineProperty(navigator, 'share', {
      value: undefined,
      writable: true,
      configurable: true,
    });

    renderDeployDisabled();

    fireEvent.click(screen.getByTestId('share-link-button'));
    expect(writeTextMock).toHaveBeenCalled();
  });
});
