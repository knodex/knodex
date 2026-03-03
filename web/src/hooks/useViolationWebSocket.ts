import { useEffect, useRef, useCallback, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type {
  WebSocketMessage,
  ViolationUpdateData,
  TemplateUpdateData,
  ConstraintUpdateData,
  ErrorData,
  ConnectionStatus,
  MessageType,
} from "@/types/websocket";
import { logger, createLogger } from "@/lib/logger";
import { getWebSocketTicket } from "@/api/auth";

// Exponential backoff configuration
const INITIAL_RECONNECT_DELAY = 1000; // 1 second
const MAX_RECONNECT_DELAY = 30000; // 30 seconds
const MAX_RECONNECT_ATTEMPTS = 10;
const PING_INTERVAL = 30000; // 30 seconds

interface UseViolationWebSocketOptions {
  /** Auto-connect on mount (default: true) */
  autoConnect?: boolean;
  /** Enable debug logging */
  debug?: boolean;
  /** Callback when a new violation is detected */
  onViolationDetected?: (violation: ViolationUpdateData) => void;
  /** Callback when a violation is resolved */
  onViolationResolved?: (violation: ViolationUpdateData) => void;
}

export interface UseViolationWebSocketReturn {
  /** Current connection status */
  status: ConnectionStatus;
  /** Connect to WebSocket server */
  connect: () => void;
  /** Disconnect from WebSocket server */
  disconnect: () => void;
  /** Last error message */
  error: string | null;
  /** Number of reconnect attempts */
  reconnectAttempts: number;
  /** Whether there was a recent real-time update */
  hasRecentUpdate: boolean;
  /** Clear the recent update indicator */
  clearRecentUpdate: () => void;
  /** Timestamp of last violation update */
  lastUpdateTime: Date | null;
}

function getWebSocketBaseUrl(): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = window.location.host;
  return `${protocol}//${host}/ws`;
}

/**
 * Custom hook for WebSocket connection with violation updates
 * Subscribes to "violations" resource type and handles real-time violation events
 */
export function useViolationWebSocket(
  options: UseViolationWebSocketOptions = {}
): UseViolationWebSocketReturn {
  const {
    autoConnect = true,
    debug = false,
    onViolationDetected,
    onViolationResolved,
  } = options;

  const [status, setStatus] = useState<ConnectionStatus>("disconnected");
  const [error, setError] = useState<string | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const [hasRecentUpdate, setHasRecentUpdate] = useState(false);
  const [lastUpdateTime, setLastUpdateTime] = useState<Date | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const recentUpdateTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isManualDisconnect = useRef(false);

  const queryClient = useQueryClient();
  const wsLogger = useMemo(() => createLogger("[ViolationWebSocket]"), []);

  const log = useCallback(
    (...args: unknown[]) => {
      if (debug) {
        wsLogger.debug(...args);
      }
    },
    [debug, wsLogger]
  );

  // Clear recent update indicator
  const clearRecentUpdate = useCallback(() => {
    setHasRecentUpdate(false);
    if (recentUpdateTimeoutRef.current) {
      clearTimeout(recentUpdateTimeoutRef.current);
      recentUpdateTimeoutRef.current = null;
    }
  }, []);

  // Set recent update indicator with auto-clear
  const setRecentUpdateWithAutoClear = useCallback((durationMs: number = 3000) => {
    setHasRecentUpdate(true);
    setLastUpdateTime(new Date());

    // Clear any existing timeout
    if (recentUpdateTimeoutRef.current) {
      clearTimeout(recentUpdateTimeoutRef.current);
    }

    // Auto-clear after duration
    recentUpdateTimeoutRef.current = setTimeout(() => {
      setHasRecentUpdate(false);
    }, durationMs);
  }, []);

  // Handle incoming messages
  const handleMessage = useCallback(
    (event: MessageEvent) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data);
        log("Received message:", message.type);

        switch (message.type) {
          case "violation_update": {
            const data = message.data as ViolationUpdateData;
            log("Violation update:", data.action, data.constraintKind, data.constraintName);

            // Set recent update indicator
            setRecentUpdateWithAutoClear();

            // Call appropriate callback
            if (data.action === "add" && onViolationDetected) {
              onViolationDetected(data);
            } else if (data.action === "delete" && onViolationResolved) {
              onViolationResolved(data);
            }

            // Show toast notification for deny violations (AC-FE-04)
            if (data.action === "add" && data.enforcementAction === "deny") {
              toast.error("Policy Violation Detected", {
                description: `${data.constraintKind}/${data.constraintName}: ${data.message}`,
                duration: 5000,
              });
            }

            // Invalidate violations query to trigger refetch
            queryClient.invalidateQueries({ queryKey: ["violations"] });
            queryClient.invalidateQueries({ queryKey: ["compliance", "violations"] });

            // Also invalidate constraints since violation counts may have changed
            queryClient.invalidateQueries({ queryKey: ["constraints"] });
            queryClient.invalidateQueries({ queryKey: ["compliance", "constraints"] });

            // Invalidate compliance summary for dashboard updates
            queryClient.invalidateQueries({ queryKey: ["compliance-summary"] });
            queryClient.invalidateQueries({ queryKey: ["compliance", "summary"] });

            break;
          }

          case "template_update": {
            const data = message.data as TemplateUpdateData;
            log("Template update:", data.action, data.name, data.kind);

            // Set recent update indicator
            setRecentUpdateWithAutoClear();

            // Invalidate templates queries to trigger refetch
            queryClient.invalidateQueries({ queryKey: ["compliance", "templates"] });
            queryClient.invalidateQueries({ queryKey: ["compliance", "template", data.name] });

            // Invalidate compliance summary for dashboard updates
            queryClient.invalidateQueries({ queryKey: ["compliance", "summary"] });

            break;
          }

          case "constraint_update": {
            const data = message.data as ConstraintUpdateData;
            log("Constraint update:", data.action, data.kind, data.name);

            // Set recent update indicator
            setRecentUpdateWithAutoClear();

            // Invalidate constraints queries to trigger refetch
            queryClient.invalidateQueries({ queryKey: ["constraints"] });
            queryClient.invalidateQueries({ queryKey: ["compliance", "constraints"] });
            queryClient.invalidateQueries({ queryKey: ["compliance", "constraint", data.kind, data.name] });

            // Invalidate compliance summary for dashboard updates
            queryClient.invalidateQueries({ queryKey: ["compliance", "summary"] });

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
            const errorData = message.data as ErrorData;
            logger.error("[ViolationWebSocket] Server error:", errorData);
            setError(errorData.message);
            break;
          }

          default:
            // Ignore other message types (instance_update, rgd_update)
            break;
        }
      } catch (err) {
        logger.error("[ViolationWebSocket] Failed to parse message:", err);
      }
    },
    [queryClient, log, onViolationDetected, onViolationResolved, setRecentUpdateWithAutoClear]
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

    // Require JWT in localStorage to prove user is authenticated before requesting ticket
    const token = localStorage.getItem("jwt_token");
    if (!token) {
      log("No authentication token available - skipping WebSocket connection");
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

          // Subscribe to violations
          sendMessage("subscribe", { resourceType: "violations" });
        };

        ws.onmessage = handleMessage;

        ws.onerror = (event) => {
          logger.error("[ViolationWebSocket] Error:", event);
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
        logger.error("[ViolationWebSocket] Failed to obtain ticket:", err);
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

    if (recentUpdateTimeoutRef.current) {
      clearTimeout(recentUpdateTimeoutRef.current);
      recentUpdateTimeoutRef.current = null;
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
    error,
    reconnectAttempts,
    hasRecentUpdate,
    clearRecentUpdate,
    lastUpdateTime,
  };
}
