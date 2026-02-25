import DOMPurify from "dompurify";

export function sanitizeRichHTML(raw: string | undefined): string {
  const source = (raw || "").trim();
  if (!source) {
    return "";
  }
  return DOMPurify.sanitize(source, { USE_PROFILES: { html: true } });
}
