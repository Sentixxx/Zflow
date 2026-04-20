import { afterEach, describe, expect, it, vi } from "vitest";
import { resolveDefaultAPIBase, resolveInitialAPIBase } from "./api-base";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

async function importWithEnv(viteApiBase: string | undefined) {
  vi.resetModules();
  if (viteApiBase === undefined) {
    vi.stubEnv("VITE_API_BASE", "");
  } else {
    vi.stubEnv("VITE_API_BASE", viteApiBase);
  }
  return import("./api-base");
}

describe("resolveDefaultAPIBase", () => {
  it("keeps localhost for local development", () => {
    expect(resolveDefaultAPIBase("localhost")).toBe("http://localhost:8080");
    expect(resolveDefaultAPIBase("127.0.0.1")).toBe("http://localhost:8080");
  });

  it("treats IPv6 loopback as localhost", () => {
    expect(resolveDefaultAPIBase("::1")).toBe("http://localhost:8080");
  });

  it("uses the current LAN host for cross-device access", () => {
    expect(resolveDefaultAPIBase("192.168.31.181")).toBe("http://192.168.31.181:8080");
  });

  it("falls back to localhost when hostname is empty or whitespace", () => {
    expect(resolveDefaultAPIBase("")).toBe("http://localhost:8080");
    expect(resolveDefaultAPIBase("   ")).toBe("http://localhost:8080");
    expect(resolveDefaultAPIBase(undefined)).toBe("http://localhost:8080");
  });

  it("normalises hostname case before deriving the base", () => {
    // Uppercase hostnames should still produce a lowercase, stable URL.
    expect(resolveDefaultAPIBase("LAN.Example.COM")).toBe("http://lan.example.com:8080");
  });

  it("is deterministic for repeated calls with the same hostname", () => {
    const host = "192.168.1.50";
    const first = resolveDefaultAPIBase(host);
    const second = resolveDefaultAPIBase(host);
    expect(first).toBe(second);
  });
});

describe("resolveInitialAPIBase", () => {
  it("prefers a non-empty stored value over hostname-derived default", () => {
    expect(resolveInitialAPIBase("http://custom.example:9000", "192.168.1.10")).toBe(
      "http://custom.example:9000",
    );
  });

  it("trims whitespace around the stored value", () => {
    expect(resolveInitialAPIBase("  http://custom.example:9000  ", "localhost")).toBe(
      "http://custom.example:9000",
    );
  });

  it("returns relative path when stored value is empty in a reverse-proxy build", () => {
    // Vitest leaves VITE_API_BASE unset, which collapses to "" — the Nginx
    // reverse-proxy contract: with no explicit stored value, the frontend
    // should emit relative URLs so the proxy resolves them against the page
    // origin. Hostname derivation is only hit when a build-time base was
    // explicitly injected as non-empty (covered indirectly by the default
    // resolver above).
    expect(resolveInitialAPIBase("", "192.168.1.10")).toBe("");
  });

  it("treats null / undefined stored value as unset (relative-path fallback)", () => {
    expect(resolveInitialAPIBase(null, "localhost")).toBe("");
    expect(resolveInitialAPIBase(undefined, "127.0.0.1")).toBe("");
  });

  it("treats a whitespace-only stored value as unset (relative-path fallback)", () => {
    expect(resolveInitialAPIBase("   ", "localhost")).toBe("");
  });
});

describe("build-time VITE_API_BASE injection", () => {
  it("honours a non-empty build-time base for resolveDefaultAPIBase", async () => {
    const mod = await importWithEnv("http://compose.example:9000");
    // The build-time base should short-circuit hostname derivation regardless
    // of what hostname the page happens to run under.
    expect(mod.resolveDefaultAPIBase("some.lan")).toBe("http://compose.example:9000");
    expect(mod.resolveDefaultAPIBase("localhost")).toBe("http://compose.example:9000");
  });

  it("resolveInitialAPIBase with a build-time base falls back to derived default when nothing is stored", async () => {
    const mod = await importWithEnv("http://compose.example:9000");
    expect(mod.resolveInitialAPIBase("", "192.168.1.10")).toBe("http://compose.example:9000");
  });

  it("resolveInitialAPIBase with a build-time base still prefers a stored value", async () => {
    const mod = await importWithEnv("http://compose.example:9000");
    expect(mod.resolveInitialAPIBase("http://override.example", "any.host")).toBe(
      "http://override.example",
    );
  });
});
