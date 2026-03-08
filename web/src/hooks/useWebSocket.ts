import { useEffect, useRef, useCallback, useState, useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { logger, createLogger } from "@/lib/logger";
import { getWebSocketTicket } from "@/api/auth";
import { useUserStore } from "@/stores/userStore";
import type {
  MessageType,
  WebSocketMessage,
  InstanceUpdateData,
  RGDUpdateData,
  CountsUpdateData,
} from "@/types/websocket";

// Re-export ConnectionStatus for existing consumers
export type { ConnectionStatus } from "@/types/websocket";

interface UseWebSocketOptions {
  /** Auto-connect on mount (default: true) */
  autoConnect?: boolean;
  /** Initial subscriptions */
  subscriptions?: string[];
  /** Enable debug logging */
  debug?: boolean;
}

export interface UseWebSocketReturn {
  /** Current connection status */
  status: ConnectionStatus;
  /** Connect to WebSocket server */
  connect: () => void;
  /** Disconnect from WebSocket server */
  disconnect: () => void;
  /** Subscribe to a resource type */
  subscribe: (resourceType: string) => void;
  /** Unsubscribe from a resource type */
  unsubscribe: (resourceType: string) => void;
  /** Last error message */
  error: string | null;
  /** Number of reconnect attempts */
  reconnectAttempts: number;
}

// Exponential backoff configuration
const INITIAL_RECONNECT_DELAY = 1000; // 1 second
const MAX_RECONNECT_DELAY = 30000; // 30 seconds
const MAX_RECONNECT_ATTEMPTS = 10;
const PING_INTERVAL = 30000; // 30 seconds

function getWebSocketBaseUrl(): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = window.location.host;
  return `${protocol}//${host}/ws`;
}

/**
 * Custom hook for WebSocket connection with auto-reconnect and React Query integration
 */
export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const { autoConnect = true, subscriptions: initialSubscriptions = [], debug = false } = options;

  const [status, setStatus] = useState<ConnectionStatus>("disconnected");
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const subscriptionsRef = useRef<Set<string>>(new Set(initialSubscriptions));
  const isManualDisconnect = useRef(false);

  const queryClient = useQueryClient();
  const wsLogger = useMemo(() => createLogger("[WebSocket]"), []);

  const log = useCallback(
    (...args: unknown[]) => {
      if (debug) {
        wsLogger.debug(...args);
      }
    },
    [debug, wsLogger]
  );

  // Handle incoming messages
  const handleMessage = useCallback(
    (event: MessageEvent) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data);
        log("Received message:", message.type);

        switch (message.type) {
          case "instance_update": {
            const data = message.data as InstanceUpdateData;
            log("Instance update:", data.action, data.namespace, data.name);

            if (data.action === "delete") {
              // Remove specific instance from cache
              queryClient.removeQueries({
                queryKey: ["instance", data.namespace, data.name],
              });
            } else if (data.instance) {
              // Update specific instance in cache
              queryClient.setQueryData(
                ["instance", data.namespace, data.name],
                data.instance
              );
            }

            // Invalidate instance list to trigger refetch, but exclude count queries
            // (counts are pushed via WebSocket counts_update, not HTTP polling)
            queryClient.invalidateQueries({
              queryKey: ["instances"],
              predicate: (query) => !query.queryKey.includes("count"),
            });
            break;
          }

          case "rgd_update": {
            const data = message.data as RGDUpdateData;
            log("RGD update:", data.action, data.name);

            // Invalidate RGD queries, but exclude count queries
            // (counts are pushed via WebSocket counts_update, not HTTP polling)
            queryClient.invalidateQueries({
              queryKey: ["rgds"],
              predicate: (query) => !query.queryKey.includes("count"),
            });
            if (data.name) {
              queryClient.invalidateQueries({ queryKey: ["rgd", data.name] });
              queryClient.invalidateQueries({ queryKey: ["rgd-schema", data.name] });
            }
            break;
          }

          case "counts_update": {
            const data = message.data as CountsUpdateData;
            log("Counts update:", data.rgdCount, "RGDs,", data.instanceCount, "instances");
            queryClient.setQueryData(["rgds", "count"], { count: data.rgdCount });
            queryClient.setQueryData(["instances", "count"], { count: data.instanceCount });
            break;
          }

          case "pong":
            log("Pong received");
            break;

          case "subscribed":
            log("Subscription confirmed:", message.data);
            break;

          case "unsubscribed":
            log("Unsubscription confirmed:", message.data);
            break;

          case "error": {
            const errorData = message.data as { code: string; message: string };
            logger.error("[WebSocket] Server error:", errorData);
            setError(errorData.message);
            break;
          }

          default:
            log("Unknown message type:", message.type);
        }
      } catch (err) {
        logger.error("[WebSocket] Failed to parse message:", err);
      }
    },
    [queryClient, log]
  );

  // Send message helper
  const sendMessage = useCallback(
    (type: MessageType, data?: unknown) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        const message = { type, data };
        wsRef.current.send(JSON.stringify(message));
        log("Sent message:", type, data);
      }
    },
    [log]
  );

  // Subscribe to resource type
  const subscribe = useCallback(
    (resourceType: string) => {
      subscriptionsRef.current.add(resourceType);
      sendMessage("subscribe", { resourceType });
    },
    [sendMessage]
  );

  // Unsubscribe from resource type
  const unsubscribe = useCallback(
    (resourceType: string) => {
      subscriptionsRef.current.delete(resourceType);
      sendMessage("unsubscribe", { resourceType });
    },
    [sendMessage]
  );

  // Start ping interval
  const startPing = useCallback(() => {
    if (pingIntervalRef.current) {
      clearInterval(pingIntervalRef.current);
    }
    pingIntervalRef.current = setInterval(() => {
      sendMessage("ping");
    }, PING_INTERVAL);
  }, [sendMessage]);

  // Stop ping interval
  const stopPing = useCallback(() => {
    if (pingIntervalRef.current) {
      clearInterval(pingIntervalRef.current);
      pingIntervalRef.current = null;
    }
  }, []);

  // Calculate reconnect delay with exponential backoff
  const getReconnectDelay = useCallback((attempt: number): number => {
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY * Math.pow(2, attempt),
      MAX_RECONNECT_DELAY
    );
    // Add jitter (0-20% of delay)
    return delay + Math.random() * delay * 0.2;
  }, []);

  // Connect to WebSocket server
  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      log("Already connected");
      return;
    }

    if (wsRef.current?.readyState === WebSocket.CONNECTING) {
      log("Connection already in progress");
      return;
    }

    // Check Zustand store for authentication state before requesting ticket.
    // The JWT is in an HttpOnly cookie (not accessible to JS), so we check the
    // store's isAuthenticated flag which is set on login.
    const isAuthenticated = useUserStore.getState().isAuthenticated;
    if (!isAuthenticated) {
      log("User not authenticated - skipping WebSocket connection");
      setStatus("disconnected");
      setError("Authentication required");
      return;
    }

    isManualDisconnect.current = false;
    setStatus("connecting");
    setError(null);

    // Fetch a single-use ticket, then open WebSocket with ?ticket=
    getWebSocketTicket()
      .then((ticket) => {
        if (isManualDisconnect.current) return;

        const url = `${getWebSocketBaseUrl()}?ticket=${encodeURIComponent(ticket)}`;
        log("Connecting with ticket");

        const ws = new WebSocket(url);
        wsRef.current = ws;

        ws.onopen = () => {
          log("Connected");
          setStatus("connected");
          setReconnectAttempts(0);
          startPing();

          // Re-subscribe to all previous subscriptions
          subscriptionsRef.current.forEach((resourceType) => {
            sendMessage("subscribe", { resourceType });
          });
        };

        ws.onmessage = handleMessage;

        ws.onerror = (event) => {
          logger.error("[WebSocket] Error:", event);
          setStatus("error");
          setError("WebSocket connection error");
        };

        ws.onclose = (event) => {
          log("Disconnected:", event.code, event.reason);
          setStatus("disconnected");
          stopPing();
          wsRef.current = null;

          // Attempt reconnection if not manually disconnected
          if (!isManualDisconnect.current && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
            const delay = getReconnectDelay(reconnectAttempts);
            log(`Reconnecting in ${Math.round(delay / 1000)}s (attempt ${reconnectAttempts + 1})`);

            reconnectTimeoutRef.current = setTimeout(() => {
              setReconnectAttempts((prev) => prev + 1);
              connect();
            }, delay);
          } else if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
            setError("Max reconnection attempts reached");
          }
        };
      })
      .catch((err) => {
        if (isManualDisconnect.current) return;
        logger.error("[WebSocket] Failed to obtain ticket:", err);
        setStatus("error");
        setError("Failed to authenticate WebSocket connection");
      });
  }, [
    handleMessage,
    log,
    sendMessage,
    startPing,
    stopPing,
    getReconnectDelay,
    reconnectAttempts,
  ]);

  // Disconnect from WebSocket server
  const disconnect = useCallback(() => {
    isManualDisconnect.current = true;

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    stopPing();

    if (wsRef.current) {
      wsRef.current.close(1000, "Client disconnect");
      wsRef.current = null;
    }

    setStatus("disconnected");
    setReconnectAttempts(0);
  }, [stopPing]);

  // Auto-connect on mount
  useEffect(() => {
    if (autoConnect) {
      connect();
    }

    return () => {
      disconnect();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    status,
    connect,
    disconnect,
    subscribe,
    unsubscribe,
    error,
    reconnectAttempts,
  };
}
