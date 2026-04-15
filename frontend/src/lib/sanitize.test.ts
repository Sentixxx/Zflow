/* @vitest-environment jsdom */
/**
 * sanitizeRichHTML contract tests.
 *
 * Invariants from CLAUDE.md and spec_rss_llm_reader.md:
 * - dangerouslySetInnerHTML must only receive DOMPurify-sanitized content.
 * - script / iframe / event-handler injection must be stripped.
 * - Safe href, img src, and semantic HTML must pass through.
 */
import { describe, expect, it } from "vitest";
import { sanitizeRichHTML } from "./sanitize";

describe("sanitizeRichHTML", () => {
  it("returns empty string for undefined input", () => {
    expect(sanitizeRichHTML(undefined)).toBe("");
  });

  it("returns empty string for empty string input", () => {
    expect(sanitizeRichHTML("")).toBe("");
  });

  it("returns empty string for whitespace-only input", () => {
    expect(sanitizeRichHTML("   ")).toBe("");
  });

  it("removes script tags to prevent XSS", () => {
    const result = sanitizeRichHTML("<p>Hello</p><script>alert('xss')</script>");
    expect(result).not.toContain("<script>");
    expect(result).not.toContain("alert");
    expect(result).toContain("Hello");
  });

  it("removes iframe tags", () => {
    const result = sanitizeRichHTML('<p>content</p><iframe src="https://evil.com"></iframe>');
    expect(result).not.toContain("<iframe");
    expect(result).toContain("content");
  });

  it("strips inline event handlers like onclick", () => {
    const result = sanitizeRichHTML('<a href="https://example.com" onclick="steal()">link</a>');
    expect(result).not.toContain("onclick");
    expect(result).toContain("link");
  });

  it("strips javascript: href protocol", () => {
    const result = sanitizeRichHTML('<a href="javascript:alert(1)">click</a>');
    // DOMPurify removes javascript: href entirely or blanks the attribute
    expect(result).not.toContain("javascript:");
  });

  it("keeps safe https href links intact", () => {
    const result = sanitizeRichHTML('<a href="https://example.com">Visit</a>');
    expect(result).toContain('href="https://example.com"');
    expect(result).toContain("Visit");
  });

  it("keeps img tags with safe src", () => {
    const result = sanitizeRichHTML('<img src="https://cdn.example.com/image.png" alt="cover">');
    expect(result).toContain("img");
    expect(result).toContain("https://cdn.example.com/image.png");
  });

  it("keeps standard semantic HTML structure", () => {
    const html = "<article><h1>Title</h1><p>Body text.</p><blockquote>Quote</blockquote></article>";
    const result = sanitizeRichHTML(html);
    expect(result).toContain("<h1>");
    expect(result).toContain("<p>");
    expect(result).toContain("<blockquote>");
    expect(result).toContain("Body text.");
  });
});
