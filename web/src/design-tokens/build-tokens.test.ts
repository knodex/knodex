// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Visual-consistency acceptance test — in-repo portion (Story 8.3 / B.3, AC #4).
 *
 * The full side-by-side visual diff across knodex-ee and knodex-cloud-v2 is the
 * Gate G5 acceptance and is deferred until Epic A renders the pre-tenant UI (see
 * design-system-bridge.md § "Visual-consistency acceptance test"). The achievable
 * in-repo portion runs here:
 *
 *   1. Zero-drift parity — the design-token values produced by the package equal
 *      a frozen pre-refactor snapshot (`__fixtures__/baseline-tokens.json`),
 *      proving knodex-ee's rendered tokens are unchanged by the package refactor.
 *      Any token-value edit is caught by this test.
 *   2. Determinism — `build:tokens` is reproducible (AC #2).
 *   3. Artifacts in sync — the committed generated artifacts match a fresh build
 *      of tokens.json, so no one can edit tokens.json without regenerating.
 *   4. CSS reproduces every baseline token verbatim (AC #3 regression guard).
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

import {
  loadTokens,
  flattenTokens,
  buildArtifacts,
  buildCss,
} from '../../scripts/build-tokens.mjs';
import baselineTokens from './__fixtures__/baseline-tokens.json';

const PACKAGE_DIR = path.dirname(fileURLToPath(import.meta.url));

/** Flatten the canonical tokens to a `{ "--name": "value" }` map. */
function tokenMap(): Record<string, string> {
  return Object.fromEntries(flattenTokens(loadTokens(PACKAGE_DIR)));
}

describe('design-tokens parity (zero visual drift)', () => {
  it('reproduces the frozen pre-refactor token snapshot exactly', () => {
    // The baseline fixture is the computed token set BEFORE the package refactor
    // (lifted verbatim from the former web/src/styles/tokens.css + index.css
    // palette primitives). Equality here is the proof of zero visual drift.
    expect(tokenMap()).toEqual(baselineTokens);
  });

  it('defines every baseline token (no token dropped by the refactor)', () => {
    const current = tokenMap();
    for (const name of Object.keys(baselineTokens)) {
      expect(current).toHaveProperty(name);
    }
  });
});

describe('build:tokens determinism (AC #2)', () => {
  it('produces byte-identical artifacts from independent loads of the source', () => {
    // Two *independent* loads (each re-reads + re-parses tokens.json, so they are
    // distinct objects) must yield byte-identical artifact strings. Asserting on
    // the string contents — not on a single in-memory object compared to itself —
    // is what proves "running it twice yields byte-identical output" (AC #2):
    // output depends only on token values and their declared order, never on
    // object identity or load timing.
    const first = buildArtifacts(loadTokens(PACKAGE_DIR));
    const second = buildArtifacts(loadTokens(PACKAGE_DIR));
    for (const relPath of Object.keys(first) as (keyof typeof first)[]) {
      expect(second[relPath]).toBe(first[relPath]);
    }
  });
});

describe('generated artifacts are in sync with tokens.json (AC #2/#3)', () => {
  const artifacts = buildArtifacts(loadTokens(PACKAGE_DIR));

  it.each(Object.keys(artifacts))(
    'committed %s matches a fresh build (run `npm run build:tokens`)',
    (relPath) => {
      const onDisk = readFileSync(path.join(PACKAGE_DIR, relPath), 'utf8');
      expect(onDisk).toBe(artifacts[relPath as keyof typeof artifacts]);
    },
  );
});

describe('css build reproduces baseline values verbatim (AC #3)', () => {
  it('emits every baseline token as a CSS custom property with its exact value', () => {
    const css = buildCss(loadTokens(PACKAGE_DIR));
    for (const [name, value] of Object.entries(baselineTokens)) {
      expect(css).toContain(`  ${name}: ${value};`);
    }
  });
});
