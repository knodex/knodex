// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { CmdKTrigger } from './cmd-k-trigger';

describe('CmdKTrigger', () => {
  it('renders with the default placeholder', () => {
    render(<CmdKTrigger onOpen={vi.fn()} />);

    const trigger = screen.getByTestId('cmdk-trigger');
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveTextContent('Search…');
  });

  it('renders with a custom placeholder', () => {
    render(<CmdKTrigger onOpen={vi.fn()} placeholder="Find anything" />);

    expect(screen.getByText('Find anything')).toBeInTheDocument();
  });

  it('invokes onOpen when clicked', () => {
    const onOpen = vi.fn();
    render(<CmdKTrigger onOpen={onOpen} />);

    fireEvent.click(screen.getByTestId('cmdk-trigger'));

    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it('exposes an aria-keyshortcuts attribute for assistive tech', () => {
    render(<CmdKTrigger onOpen={vi.fn()} />);

    const trigger = screen.getByTestId('cmdk-trigger');
    const value = trigger.getAttribute('aria-keyshortcuts');
    expect(value).toMatch(/(Meta|Control)\+K/);
  });

  it('renders a keyboard shortcut hint inside a <kbd> element', () => {
    render(<CmdKTrigger onOpen={vi.fn()} />);

    const trigger = screen.getByTestId('cmdk-trigger');
    const kbd = trigger.querySelector('kbd');
    expect(kbd).toBeTruthy();
    // Hint copy depends on the test runner's reported platform — either ⌘K or Ctrl K
    expect(kbd?.textContent).toMatch(/⌘K|Ctrl K/);
  });

  // Regression guard: the platform check must run via useState's lazy
  // initializer so first paint already reflects the Mac shortcut. Reverting
  // to `useState(false) + useEffect` would re-introduce a one-frame flash of
  // the wrong hint on macOS.
  it('reflects the macOS shortcut synchronously on first paint', () => {
    const originalPlatform = Object.getOwnPropertyDescriptor(
      window.navigator,
      'platform',
    );
    Object.defineProperty(window.navigator, 'platform', {
      value: 'MacIntel',
      configurable: true,
      writable: true,
    });

    try {
      render(<CmdKTrigger onOpen={vi.fn()} />);
      const kbd = screen.getByTestId('cmdk-trigger').querySelector('kbd');
      expect(kbd?.textContent).toBe('⌘K');
      expect(screen.getByTestId('cmdk-trigger')).toHaveAttribute(
        'aria-keyshortcuts',
        'Meta+K',
      );
    } finally {
      if (originalPlatform) {
        Object.defineProperty(window.navigator, 'platform', originalPlatform);
      }
    }
  });

  it('reflects the non-macOS shortcut on first paint', () => {
    const originalPlatform = Object.getOwnPropertyDescriptor(
      window.navigator,
      'platform',
    );
    Object.defineProperty(window.navigator, 'platform', {
      value: 'Linux x86_64',
      configurable: true,
      writable: true,
    });

    try {
      render(<CmdKTrigger onOpen={vi.fn()} />);
      const kbd = screen.getByTestId('cmdk-trigger').querySelector('kbd');
      expect(kbd?.textContent).toBe('Ctrl K');
      expect(screen.getByTestId('cmdk-trigger')).toHaveAttribute(
        'aria-keyshortcuts',
        'Control+K',
      );
    } finally {
      if (originalPlatform) {
        Object.defineProperty(window.navigator, 'platform', originalPlatform);
      }
    }
  });
});
