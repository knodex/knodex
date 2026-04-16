// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { orderEntries, orderProperties } from "./order-properties";
import type { FormProperty } from "@/types/rgd";

const makeProp = (type: string): FormProperty => ({ type });

describe("orderEntries", () => {
  it("returns entries in annotation order when propertyOrder is provided", () => {
    const entries: [string, unknown][] = [
      ["charlie", 3],
      ["alpha", 1],
      ["bravo", 2],
    ];
    const result = orderEntries(entries, ["alpha", "bravo", "charlie"]);
    expect(result.map(([k]) => k)).toEqual(["alpha", "bravo", "charlie"]);
  });

  it("appends unlisted fields alphabetically after ordered fields", () => {
    const entries: [string, unknown][] = [
      ["zulu", 4],
      ["alpha", 1],
      ["mike", 3],
      ["bravo", 2],
    ];
    const result = orderEntries(entries, ["bravo", "alpha"]);
    expect(result.map(([k]) => k)).toEqual(["bravo", "alpha", "mike", "zulu"]);
  });

  it("returns alphabetical order when propertyOrder is undefined", () => {
    const entries: [string, unknown][] = [
      ["charlie", 3],
      ["alpha", 1],
      ["bravo", 2],
    ];
    const result = orderEntries(entries, undefined);
    expect(result.map(([k]) => k)).toEqual(["alpha", "bravo", "charlie"]);
  });

  it("returns alphabetical order when propertyOrder is empty array", () => {
    const entries: [string, unknown][] = [
      ["charlie", 3],
      ["alpha", 1],
    ];
    const result = orderEntries(entries, []);
    expect(result.map(([k]) => k)).toEqual(["alpha", "charlie"]);
  });

  it("ignores annotation fields not present in entries", () => {
    const entries: [string, unknown][] = [
      ["bravo", 2],
      ["alpha", 1],
    ];
    const result = orderEntries(entries, ["missing", "alpha", "bravo"]);
    expect(result.map(([k]) => k)).toEqual(["alpha", "bravo"]);
  });

  it("handles single-field and empty entries arrays", () => {
    expect(orderEntries([], ["a"])).toEqual([]);
    const single: [string, unknown][] = [["only", 1]];
    expect(orderEntries(single, ["only"]).map(([k]) => k)).toEqual(["only"]);
  });

  it("deduplicates: first occurrence wins for duplicate propertyOrder entries", () => {
    const entries: [string, unknown][] = [
      ["bravo", 2],
      ["alpha", 1],
      ["charlie", 3],
    ];
    // "alpha" appears at index 0 and 2 — first occurrence (index 0) wins
    const result = orderEntries(entries, ["alpha", "charlie", "alpha"]);
    expect(result.map(([k]) => k)).toEqual(["alpha", "charlie", "bravo"]);
  });

  it("does not mutate the original input array", () => {
    const entries: [string, unknown][] = [
      ["charlie", 3],
      ["alpha", 1],
      ["bravo", 2],
    ];
    const original = [...entries];
    orderEntries(entries, ["bravo", "alpha"]);
    expect(entries).toEqual(original);
  });

  it("works with non-FormProperty generic types", () => {
    const entries: [string, number][] = [
      ["z", 26],
      ["a", 1],
      ["m", 13],
    ];
    const result = orderEntries(entries, ["m", "a"]);
    expect(result).toEqual([
      ["m", 13],
      ["a", 1],
      ["z", 26],
    ]);
  });
});

describe("orderProperties", () => {
  it("orders FormProperty entries by propertyOrder", () => {
    const entries: [string, FormProperty][] = [
      ["port", makeProp("integer")],
      ["name", makeProp("string")],
      ["image", makeProp("string")],
    ];
    const result = orderProperties(entries, ["name", "image", "port"]);
    expect(result.map(([k]) => k)).toEqual(["name", "image", "port"]);
  });

  it("falls back to alphabetical when no propertyOrder", () => {
    const entries: [string, FormProperty][] = [
      ["port", makeProp("integer")],
      ["name", makeProp("string")],
    ];
    const result = orderProperties(entries);
    expect(result.map(([k]) => k)).toEqual(["name", "port"]);
  });
});
