import { describe, expect, test } from "vitest";
import {
  formatBps,
  formatBytes,
  formatDuration,
  formatPercent,
  formatRelativeAgo,
} from "./formatters";

describe("formatBps", () => {
  test("small", () => expect(formatBps(500)).toBe("500 bps"));
  test("mbps", () => expect(formatBps(1_400_000)).toBe("1.4 Mbps"));
  test("zero", () => expect(formatBps(0)).toBe("0 bps"));
});

describe("formatBytes", () => {
  test("KiB", () => expect(formatBytes(2048)).toBe("2.0 KiB"));
  test("zero", () => expect(formatBytes(0)).toBe("0 B"));
});

describe("formatDuration", () => {
  test("days", () =>
    expect(formatDuration(14 * 86400 + 2 * 3600)).toBe("14d 2h"));
  test("seconds", () => expect(formatDuration(17)).toBe("17s"));
});

describe("formatPercent", () => {
  test("fraction", () => expect(formatPercent(0.34)).toBe("34%"));
  test("whole", () => expect(formatPercent(34)).toBe("34%"));
});

describe("formatRelativeAgo", () => {
  test("null", () => expect(formatRelativeAgo(null)).toBe("never"));
});
