type LogLevel = "debug" | "info" | "warn" | "error";

function shouldLog(level: LogLevel): boolean {
  if (level === "warn" || level === "error") {
    return true;
  }
  return import.meta.env.DEV;
}

function safeMeta(meta?: Record<string, unknown>): Record<string, unknown> | undefined {
  if (!meta) {
    return undefined;
  }
  const next: Record<string, unknown> = {};
  Object.entries(meta).forEach(([key, value]) => {
    if (value == null) {
      return;
    }
    next[key] = value;
  });
  return Object.keys(next).length > 0 ? next : undefined;
}

export function createLogger(module: string) {
  const prefix = `[${module}]`;

  const emit = (level: LogLevel, message: string, meta?: Record<string, unknown>) => {
    if (!shouldLog(level)) {
      return;
    }
    const payload = safeMeta(meta);
    if (level === "debug") {
      console.debug(prefix, message, payload ?? "");
      return;
    }
    if (level === "info") {
      console.info(prefix, message, payload ?? "");
      return;
    }
    if (level === "warn") {
      console.warn(prefix, message, payload ?? "");
      return;
    }
    console.error(prefix, message, payload ?? "");
  };

  return {
    debug: (message: string, meta?: Record<string, unknown>) => emit("debug", message, meta),
    info: (message: string, meta?: Record<string, unknown>) => emit("info", message, meta),
    warn: (message: string, meta?: Record<string, unknown>) => emit("warn", message, meta),
    error: (message: string, meta?: Record<string, unknown>) => emit("error", message, meta),
  };
}
