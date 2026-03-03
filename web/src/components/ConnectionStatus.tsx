import { useWebSocketContext, type ConnectionStatus as Status } from "@/context/WebSocketContext";
import { cn } from "@/lib/utils";
import { Wifi, WifiOff, Loader2, AlertCircle } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface ConnectionStatusProps {
  /** Show full label or just dot */
  showLabel?: boolean;
  /** Additional CSS classes */
  className?: string;
}

const statusConfig: Record<Status, {
  label: string;
  color: string;
  icon: typeof Wifi;
  animate?: boolean;
}> = {
  connected: {
    label: "Connected",
    color: "text-primary",
    icon: Wifi,
  },
  connecting: {
    label: "Connecting",
    color: "text-warning",
    icon: Loader2,
    animate: true,
  },
  disconnected: {
    label: "Disconnected",
    color: "text-muted-foreground",
    icon: WifiOff,
  },
  error: {
    label: "Connection Error",
    color: "text-destructive",
    icon: AlertCircle,
  },
};

/**
 * Displays WebSocket connection status with icon and optional label
 */
export function ConnectionStatus({ showLabel = true, className }: ConnectionStatusProps) {
  const { status, reconnectAttempts, error } = useWebSocketContext();
  const config = statusConfig[status];
  const Icon = config.icon;

  // Build tooltip text
  let tooltip = config.label;
  if (status === "connecting" && reconnectAttempts > 0) {
    tooltip = `Reconnecting (attempt ${reconnectAttempts})`;
  } else if (status === "error" && error) {
    tooltip = `Error: ${error}`;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          className={cn("flex items-center gap-2 text-sm cursor-default", className)}
        >
          <span className={cn("relative flex items-center", config.color)}>
            <Icon
              className={cn(
                "h-4 w-4",
                config.animate && "animate-spin"
              )}
            />
            {status === "connected" && (
              <span className="absolute -top-0.5 -right-0.5 h-2 w-2 rounded-full bg-primary animate-pulse" />
            )}
          </span>
          {showLabel && (
            <span className={cn("hidden sm:inline", config.color)}>
              {status === "connecting" && reconnectAttempts > 0
                ? `Reconnecting (${reconnectAttempts})`
                : config.label}
            </span>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <p>{tooltip}</p>
      </TooltipContent>
    </Tooltip>
  );
}

/**
 * Simple status dot indicator
 */
export function ConnectionDot({ className }: { className?: string }) {
  const { status } = useWebSocketContext();

  const dotColor = {
    connected: "bg-primary",
    connecting: "bg-warning animate-pulse",
    disconnected: "bg-muted-foreground",
    error: "bg-destructive",
  }[status];

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className={cn(
            "inline-block h-2 w-2 rounded-full cursor-default",
            dotColor,
            className
          )}
        />
      </TooltipTrigger>
      <TooltipContent>
        <p>{statusConfig[status].label}</p>
      </TooltipContent>
    </Tooltip>
  );
}
