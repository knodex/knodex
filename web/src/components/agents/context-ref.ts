// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * contextRef parsing for agent run records (Story 49.4). The linking
 * convention is a LOCKED forward contract for 50.x:
 *   - "instance:{group}/{version}/{namespace}/{kind}/{name}" → instance detail
 *   - "rgd:{name}" → catalog detail
 *   - anything else (or empty) renders as plain truncated text
 */

/** Parsed contextRef link target. */
export type ParsedContextRef =
  | { kind: "instance" | "rgd"; to: string; label: string }
  | { kind: "text"; label: string };

/** Pure contextRef parser. */
export function parseContextRef(contextRef: string): ParsedContextRef {
  if (contextRef.startsWith("instance:")) {
    const rest = contextRef.slice("instance:".length);
    const parts = rest.split("/");
    // Namespaced instance route shape: /instances/:group/:version/:namespace/:kind/:name
    if (parts.length === 5 && parts.every((p) => p.length > 0)) {
      return {
        kind: "instance",
        to: `/instances/${parts.map(encodeURIComponent).join("/")}`,
        label: `${parts[3]}/${parts[4]}`,
      };
    }
    return { kind: "text", label: contextRef };
  }
  if (contextRef.startsWith("rgd:")) {
    const name = contextRef.slice("rgd:".length);
    if (name.length > 0 && !name.includes("/")) {
      return { kind: "rgd", to: `/catalog/${encodeURIComponent(name)}`, label: name };
    }
    return { kind: "text", label: contextRef };
  }
  return { kind: "text", label: contextRef };
}
