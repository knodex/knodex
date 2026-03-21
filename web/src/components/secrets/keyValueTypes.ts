// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

export interface KeyValuePair {
  id: number;
  key: string;
  value: string;
  visible: boolean;
}

let nextPairId = 0;
export function createPairId(): number {
  return nextPairId++;
}
