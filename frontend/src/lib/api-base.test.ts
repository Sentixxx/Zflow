import { describe, expect, it } from "vitest";
import { resolveDefaultAPIBase } from "./api-base";

describe("resolveDefaultAPIBase", () => {
  it("keeps localhost for local development", () => {
    expect(resolveDefaultAPIBase("localhost")).toBe("http://localhost:8080");
    expect(resolveDefaultAPIBase("127.0.0.1")).toBe("http://localhost:8080");
  });

  it("uses the current LAN host for cross-device access", () => {
    expect(resolveDefaultAPIBase("192.168.31.181")).toBe("http://192.168.31.181:8080");
  });
});
