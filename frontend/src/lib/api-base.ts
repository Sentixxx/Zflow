export function resolveDefaultAPIBase(hostname?: string): string {
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
  return resolveDefaultAPIBase(hostname);
}
