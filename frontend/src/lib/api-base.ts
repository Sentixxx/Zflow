// VITE_API_BASE is injected at build time.
//   unset               → dev: derive from page hostname (http://<host>:8080)
//   ""                  → prod build behind a reverse proxy: use relative paths
//   "http://host:port"  → explicit absolute base
const BUILD_TIME_API_BASE: string = import.meta.env.VITE_API_BASE ?? "";

export function resolveDefaultAPIBase(hostname?: string): string {
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
  if (BUILD_TIME_API_BASE === "" && !import.meta.env.DEV) {
    // Prod build with empty base → reverse-proxy deploy, emit relative URLs.
    return "";
  }
  return resolveDefaultAPIBase(hostname);
}
