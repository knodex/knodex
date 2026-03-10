// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { createContext, useContext, type ReactNode } from "react";
import { useWebSocket, type ConnectionStatus, type UseWebSocketReturn } from "@/hooks/useWebSocket";

type WebSocketContextValue = UseWebSocketReturn;

const WebSocketContext = createContext<WebSocketContextValue | null>(null);

interface WebSocketProviderProps {
  children: ReactNode;
  /** Enable debug logging */
  debug?: boolean;
  /** Auto-connect on mount (default: true) */
  autoConnect?: boolean;
}

/**
 * Provides WebSocket connection context to the application
 */
export function WebSocketProvider({
  children,
  debug = false,
  autoConnect = true,
}: WebSocketProviderProps) {
  const ws = useWebSocket({
    autoConnect,
    debug,
    subscriptions: ["instances", "rgds"],
  });

  return (
    <WebSocketContext.Provider value={ws}>
      {children}
    </WebSocketContext.Provider>
  );
}

/**
 * Hook to access WebSocket connection from context
 */
// eslint-disable-next-line react-refresh/only-export-components -- Context hook must be exported with provider
export function useWebSocketContext(): WebSocketContextValue {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error("useWebSocketContext must be used within a WebSocketProvider");
  }
  return context;
}

export type { ConnectionStatus };
