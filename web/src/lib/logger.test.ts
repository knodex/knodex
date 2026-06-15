// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { logger, createLogger } from "./logger";

describe("logger", () => {
  beforeEach(() => {
    vi.spyOn(console, "log").mockImplementation(() => {});
    vi.spyOn(console, "info").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("always logs warn and error", () => {
    logger.warn("w");
    logger.error("e");
    expect(console.warn).toHaveBeenCalledWith("w");
    expect(console.error).toHaveBeenCalledWith("e");
  });

  it("routes debug/info/log through their respective console methods (dev gate)", () => {
    // In the test env import.meta.env.DEV is true, so these emit.
    logger.debug("d");
    logger.info("i");
    logger.log("l");
    expect(console.log).toHaveBeenCalledWith("d");
    expect(console.info).toHaveBeenCalledWith("i");
    expect(console.log).toHaveBeenCalledWith("l");
  });

  describe("createLogger", () => {
    it("prefixes every level with the given tag", () => {
      const log = createLogger("[WS]");
      log.debug("connected");
      log.info("info");
      log.warn("warn");
      log.error("err");
      log.log("alias");

      expect(console.warn).toHaveBeenCalledWith("[WS]", "warn");
      expect(console.error).toHaveBeenCalledWith("[WS]", "err");
      expect(console.log).toHaveBeenCalledWith("[WS]", "connected");
      expect(console.log).toHaveBeenCalledWith("[WS]", "alias");
      expect(console.info).toHaveBeenCalledWith("[WS]", "info");
    });
  });
});
