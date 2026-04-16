// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import type { SessionStatus } from '@/stores/userStore';

// Mock useSessionRestore as a no-op
vi.mock('@/hooks/useSessionRestore', () => ({
  useSessionRestore: vi.fn(),
}));

// Mock WebSocketProvider and useWebSocketContext
vi.mock('@/context', () => ({
  WebSocketProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useWebSocketContext: () => ({ status: 'disconnected', error: null }),
}));

// Mock layout sub-components
vi.mock('@/components/layout', () => ({
  Sidebar: () => <div data-testid="sidebar">Sidebar</div>,
  TopBar: () => <div data-testid="topbar">TopBar</div>,
  Breadcrumbs: () => <div data-testid="breadcrumbs">Breadcrumbs</div>,
}));

// Mock accessibility components
vi.mock('@/components/accessibility', () => ({
  Announcer: () => null,
}));

vi.mock('@/hooks/useAnnouncements', () => ({
  useAnnouncements: () => ({
    announcements: [],
    announce: vi.fn(),
    handleAnnouncementRead: vi.fn(),
  }),
}));

// Controllable session state
let mockSessionStatus: SessionStatus = 'valid';
let mockSessionError: string | null = null;
let mockHasPersistedSession = true;
const mockSetSessionStatus = vi.fn((status: SessionStatus) => {
  mockSessionStatus = status;
});

vi.mock('@/hooks/useAuth', () => ({
  hasPersistedSession: () => mockHasPersistedSession,
  useSessionStatus: () => mockSessionStatus,
  useSessionError: () => mockSessionError,
}));

vi.mock('@/stores/userStore', () => ({
  useUserStore: Object.assign(
    (selector: (state: Record<string, unknown>) => unknown) =>
      selector({
        sessionStatus: mockSessionStatus,
        sessionError: mockSessionError,
      }),
    {
      getState: () => ({
        sessionStatus: mockSessionStatus,
        setSessionStatus: mockSetSessionStatus,
      }),
    }
  ),
}));

// Mock command palette (uses React Query which isn't in test tree)
vi.mock('@/components/command-palette/command-palette', () => ({
  CommandPalette: () => null,
}));
vi.mock('@/components/command-palette/use-command-palette-shortcut', () => ({
  useCommandPaletteShortcut: () => ({ open: false, setOpen: vi.fn() }),
}));

// Import after mocks
import { DashboardLayout } from './DashboardLayout';

function renderDashboard(initialPath = '/dashboard') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route element={<DashboardLayout />}>
          <Route path="/dashboard" element={<div>Dashboard Content</div>} />
        </Route>
        <Route path="/login" element={<div>Login Page</div>} />
      </Routes>
    </MemoryRouter>
  );
}

describe('DashboardLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSessionStatus = 'valid';
    mockSessionError = null;
    mockHasPersistedSession = true;
  });

  it('renders outlet content when session is valid', () => {
    renderDashboard();

    expect(screen.getByText('Dashboard Content')).toBeInTheDocument();
    expect(screen.getByTestId('sidebar')).toBeInTheDocument();
    expect(screen.getByTestId('topbar')).toBeInTheDocument();
  });

  it('renders loading spinner when session is idle', () => {
    mockSessionStatus = 'idle';

    renderDashboard();

    // Should show spinner, not content
    expect(screen.queryByText('Dashboard Content')).not.toBeInTheDocument();
    // Sidebar/topbar should still render (dashboard chrome)
    expect(screen.getByTestId('sidebar')).toBeInTheDocument();
  });

  it('renders loading spinner when session is validating', () => {
    mockSessionStatus = 'validating';

    renderDashboard();

    expect(screen.queryByText('Dashboard Content')).not.toBeInTheDocument();
    expect(screen.getByTestId('sidebar')).toBeInTheDocument();
  });

  it('renders error state with retry button when session has error', () => {
    mockSessionStatus = 'error';
    mockSessionError = 'Server is down';

    renderDashboard();

    expect(screen.getByText('Connection Error')).toBeInTheDocument();
    expect(screen.getByText('Server is down')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    expect(screen.queryByText('Dashboard Content')).not.toBeInTheDocument();
  });

  it('renders default error message when sessionError is null', () => {
    mockSessionStatus = 'error';
    mockSessionError = null;

    renderDashboard();

    expect(screen.getByText('Unable to connect to server.')).toBeInTheDocument();
  });

  it('retry button resets session status to idle', () => {
    mockSessionStatus = 'error';
    mockSessionError = 'Connection failed';

    renderDashboard();

    fireEvent.click(screen.getByRole('button', { name: /retry/i }));

    expect(mockSetSessionStatus).toHaveBeenCalledWith('idle');
  });

  it('redirects to login when no persisted session', () => {
    mockHasPersistedSession = false;

    renderDashboard();

    expect(screen.getByText('Login Page')).toBeInTheDocument();
    expect(screen.queryByText('Dashboard Content')).not.toBeInTheDocument();
  });

  it('redirects to login when session status is logged_out', () => {
    mockSessionStatus = 'logged_out';

    renderDashboard();

    expect(screen.getByText('Login Page')).toBeInTheDocument();
  });

  describe('Accessibility - Skip Link', () => {
    it('renders skip link with correct href', () => {
      renderDashboard();

      const skipLink = screen.getByText('Skip to main content');
      expect(skipLink).toBeInTheDocument();
      expect(skipLink.tagName).toBe('A');
      expect(skipLink).toHaveAttribute('href', '#main-content');
    });

    it('skip link is the first focusable element in the DOM', () => {
      renderDashboard();

      const allFocusable = document.querySelectorAll<HTMLElement>(
        'a[href], button, input, textarea, select, [tabindex]:not([tabindex="-1"])'
      );
      expect(allFocusable.length).toBeGreaterThan(0);
      expect(allFocusable[0]).toHaveTextContent('Skip to main content');
    });

    it('skip link targets main content area', () => {
      renderDashboard();

      const main = document.getElementById('main-content');
      expect(main).toBeInTheDocument();
      expect(main?.tagName).toBe('MAIN');
    });
  });

  describe('Accessibility - Focus management setup', () => {
    it('main content has tabIndex=-1 for programmatic focus', () => {
      renderDashboard();

      const main = document.getElementById('main-content');
      expect(main).toHaveAttribute('tabindex', '-1');
    });

    it('does not steal focus on initial page load', async () => {
      renderDashboard();

      // Wait past the 100ms timeout — focus should NOT move on initial render
      await new Promise((r) => setTimeout(r, 200));

      // Focus should not be on main (isFirstRender ref prevents it)
      const main = document.getElementById('main-content');
      expect(document.activeElement).not.toBe(main);
    });
  });
});
