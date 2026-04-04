// VITE_API_BASE is injected at build time. When building for Docker Compose
// (Nginx reverse proxy), set VITE_API_BASE="" so all /api requests go to the
// current origin. For local dev, leave it unset to fall back to :8080.
const BUILD_TIME_API_BASE: string = import.meta.env.VITE_API_BASE ?? "";

export function resolveDefaultAPIBase(hostname?: string): string {
  // If a build-time base was injected (e.g. empty string for reverse-proxy
  // deployments), honour it over the hostname-based heuristic.
  if (BUILD_TIME_API_BASE !== "") {
    return BUILD_TIME_API_BASE;
  }
  const currentHost = (hostname || "").trim().toLowerCase();
  if (!currentHost || currentHost === "localhost" || currentHost === "127.0.0.1" || currentHost === "::1") {
    return "http://localhost:8080";
  }
  return `http://${currentHost}:8080`;
}

export function resolveInitialAPIBase(storedValue?: string | null, hostname?: string): string {
  const saved = (storedValue || "").trim();
  if (saved) {
    return saved;
  }
  // In Docker Compose (Nginx reverse proxy) BUILD_TIME_API_BASE is "", which
  // means "use relative paths" — the API client prepends nothing, so requests
  // go to the same origin as the page (handled by Nginx).
  if (BUILD_TIME_API_BASE === "") {
    return "";
  }
  return resolveDefaultAPIBase(hostname);
}
