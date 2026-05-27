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

  it("falls back to hostname-derived base in dev mode when VITE_API_BASE is unset", async () => {
    // Dev contract: `npm run dev` leaves VITE_API_BASE unset. There is no
    // reverse proxy, so the frontend must talk to the Go backend on :8080.
    // Relative paths would hit Vite (404).
    const mod = await importWithEnv(undefined);
    expect(mod.resolveInitialAPIBase("", "192.168.1.10")).toBe("http://192.168.1.10:8080");
    expect(mod.resolveInitialAPIBase("", "localhost")).toBe("http://localhost:8080");
    expect(mod.resolveInitialAPIBase(null, "127.0.0.1")).toBe("http://localhost:8080");
    expect(mod.resolveInitialAPIBase("   ", "localhost")).toBe("http://localhost:8080");
    expect(mod.resolveInitialAPIBase(undefined, "localhost")).toBe("http://localhost:8080");
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

  it("returns relative path in a prod reverse-proxy build (DEV=false, base empty)", async () => {
    vi.stubEnv("VITE_API_BASE", "");
    vi.stubEnv("DEV", false);
    vi.resetModules();
    const mod = await import("./api-base");
    expect(mod.resolveInitialAPIBase("", "192.168.1.10")).toBe("");
    expect(mod.resolveInitialAPIBase(null, "localhost")).toBe("");
    expect(mod.resolveInitialAPIBase("   ", "127.0.0.1")).toBe("");
  });
});
