// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Component, type ReactNode } from "react";
import { useNavigate, useLocation, type NavigateFunction } from "react-router-dom";
import { AlertTriangle, ArrowLeft, RefreshCw } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { logger } from "@/lib/logger";

interface RouteErrorBoundaryProps {
  children: ReactNode;
  /** Optional custom fallback UI */
  fallback?: ReactNode;
}

interface InternalProps extends RouteErrorBoundaryProps {
  navigate: NavigateFunction;
  /** Current route path for contextual error logging */
  routePath: string;
}

interface RouteErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

class RouteErrorBoundaryInner extends Component<InternalProps, RouteErrorBoundaryState> {
  constructor(props: InternalProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): RouteErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo): void {
    logger.error(`[RouteErrorBoundary:${this.props.routePath}] Caught error:`, error, errorInfo);
  }

  handleRetry = (): void => {
    this.setState({ hasError: false, error: null });
  };

  handleGoBack = (): void => {
    if (window.history.length <= 1) {
      this.props.navigate("/instances", { replace: true });
    } else {
      this.props.navigate(-1);
    }
  };

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="flex items-center justify-center p-8" data-testid="route-error-boundary">
          <Card className="max-w-md w-full">
            <CardHeader className="text-center">
              <div className="mx-auto mb-3 w-12 h-12 rounded-full bg-destructive/10 flex items-center justify-center">
                <AlertTriangle className="h-6 w-6 text-destructive" />
              </div>
              <CardTitle className="text-lg">Something went wrong</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground text-center mb-4">
                An error occurred in this section. Other parts of the app are unaffected.
              </p>
              {this.state.error && (
                <div className="rounded-lg border border-border bg-secondary/30 p-3">
                  <p className="text-xs font-mono text-muted-foreground break-all">
                    {this.state.error.message}
                  </p>
                </div>
              )}
            </CardContent>
            <CardFooter className="justify-center gap-3">
              <Button variant="outline" size="sm" onClick={this.handleGoBack} className="gap-2">
                <ArrowLeft className="h-4 w-4" />
                Go Back
              </Button>
              <Button variant="default" size="sm" onClick={this.handleRetry} className="gap-2">
                <RefreshCw className="h-4 w-4" />
                Retry
              </Button>
            </CardFooter>
          </Card>
        </div>
      );
    }

    return this.props.children;
  }
}

/**
 * Route-level error boundary that renders an inline error card instead of
 * crashing the entire page. Keeps sidebar/header visible so the user can
 * navigate away without a full reload.
 *
 * Uses a functional wrapper to access React Router's useNavigate hook,
 * which is unavailable in class components directly.
 */
export function RouteErrorBoundary({ children, fallback }: RouteErrorBoundaryProps) {
  const navigate = useNavigate();
  const location = useLocation();
  return (
    <RouteErrorBoundaryInner navigate={navigate} routePath={location.pathname} fallback={fallback}>
      {children}
    </RouteErrorBoundaryInner>
  );
}
