/* @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import { buildTranslationTemplate, renderTranslatedHTML, splitTranslatedTextBlocks } from "./translation";

describe("splitTranslatedTextBlocks", () => {
  it("splits translated text into safe display blocks on blank lines", () => {
    const input = [
      "## 小结",
      "- 第一条",
      "- 第二条",
      "",
      "这是一段很长的译文，应该作为另一块内容显示。",
    ].join("\n");

    expect(splitTranslatedTextBlocks(input)).toEqual([
      "## 小结\n- 第一条\n- 第二条",
      "这是一段很长的译文，应该作为另一块内容显示。",
    ]);
  });

  it("drops empty whitespace-only blocks", () => {
    expect(splitTranslatedTextBlocks("\n\n  \n译文正文\n\n")).toEqual(["译文正文"]);
  });
});

describe("buildTranslationTemplate", () => {
  it("keeps original structure and extracts translatable sources by index", () => {
    const template = buildTranslationTemplate(
      '<div class="article-body"><p>First paragraph</p><figure class="hero"><img src="/cover.png" alt="cover"></figure><p>Second paragraph</p></div>',
    );

    expect(template.sources).toEqual(["First paragraph", "Second paragraph"]);
    expect(template.html).toContain('<div class="article-body">');
    expect(template.html).toContain('<figure class="hero"><img src="/cover.png" alt="cover"></figure>');
    expect(template.html).toContain('data-translation-index="1"');
    expect(template.html).toContain('data-translation-index="2"');
  });
});

describe("renderTranslatedHTML", () => {
  it("appends translation blocks below matching nodes without losing wrappers or media", () => {
    const template = buildTranslationTemplate(
      '<div class="article-body"><p>First paragraph</p><figure class="hero"><img src="/cover.png" alt="cover"></figure><p>Second paragraph</p></div>',
    );

    const rendered = renderTranslatedHTML(
      template.html,
      [
        { index: 1, translated: "第一段译文", status: "done" },
        { index: 2, translated: "第二段译文", status: "done" },
      ],
      false,
    );

    expect(rendered).toContain('<div class="article-body">');
    expect(rendered).toContain('<figure class="hero"><img src="/cover.png" alt="cover"></figure>');
    expect(rendered).toContain("第一段译文");
    expect(rendered).toContain("第二段译文");
    expect(rendered.match(/translation-block/g)?.length).toBe(2);
    expect(rendered.match(/immersive-translation/g)?.length).toBeGreaterThanOrEqual(2);
  });
});
