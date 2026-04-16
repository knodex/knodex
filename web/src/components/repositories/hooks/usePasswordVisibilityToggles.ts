// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";

/**
 * Hook encapsulating show/hide state for password-like fields.
 */
export function usePasswordVisibilityToggles() {
  const [showPrivateKey, setShowPrivateKey] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showBearerToken, setShowBearerToken] = useState(false);

  const togglePrivateKey = useCallback(() => setShowPrivateKey((prev) => !prev), []);
  const togglePassword = useCallback(() => setShowPassword((prev) => !prev), []);
  const toggleBearerToken = useCallback(() => setShowBearerToken((prev) => !prev), []);

  return {
    showPrivateKey, togglePrivateKey,
    showPassword, togglePassword,
    showBearerToken, toggleBearerToken,
  };
}
