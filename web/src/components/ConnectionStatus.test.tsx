// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TooltipProvider } from '@/components/ui/tooltip';
import { ConnectionStatus } from './ConnectionStatus';

const mockUseWebSocketContext = vi.fn();
vi.mock('@/context/WebSocketContext', () => ({
  useWebSocketContext: () => mockUseWebSocketContext(),
}));

function wrapper(ui: React.ReactNode) {
  return render(<TooltipProvider>{ui}</TooltipProvider>);
}

describe('ConnectionStatus default variant', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the icon-and-label layout when connected', () => {
    mockUseWebSocketContext.mockReturnValue({
      status: 'connected',
      error: null,
      reconnectAttempts: 0,
    });

    wrapper(<ConnectionStatus />);

    expect(screen.getByText('Connected')).toBeInTheDocument();
  });

  it('renders the reconnecting label with attempt count', () => {
    mockUseWebSocketContext.mockReturnValue({
      status: 'connecting',
      error: null,
      reconnectAttempts: 2,
    });

    wrapper(<ConnectionStatus />);

    expect(screen.getByText(/Reconnecting \(2\)/)).toBeInTheDocument();
  });

  it('renders the error state label', () => {
    mockUseWebSocketContext.mockReturnValue({
      status: 'error',
      error: 'boom',
      reconnectAttempts: 0,
    });

    wrapper(<ConnectionStatus />);

    expect(screen.getByText('Connection Error')).toBeInTheDocument();
  });
});
