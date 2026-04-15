/* @vitest-environment jsdom */
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
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

describe("buildTranslationTemplate — code block skipping", () => {
  it("does not annotate <pre><code> blocks", () => {
    const template = buildTranslationTemplate(
      '<div><p>Normal text</p><pre><code>const x = 1;\nconst y = 2;</code></pre></div>',
    );
    // Only the <p> should be a source; the pre/code must be skipped entirely
    expect(template.sources).toEqual(["Normal text"]);
    expect(template.html).not.toContain("data-translation-index=\"2\"");
    // The code content must be unchanged
    expect(template.html).toContain("const x = 1;");
  });

  it("does not annotate bare <code> inline elements", () => {
    const template = buildTranslationTemplate(
      '<p>Use the <code>npm install</code> command.</p>',
    );
    // The entire <p> is the translation target (contains a code child, but
    // isTranslationTarget uses TRANSLATABLE_TAGS and the <code> tag is not in there
    // after our fix — the <p> itself must be annotated, but the code text must not
    // appear as a separate source)
    expect(template.sources.length).toBe(1);
    // Verify no data-translation-index on the code element
    const container = document.createElement("div");
    container.innerHTML = template.html;
    const code = container.querySelector("code");
    expect(code?.hasAttribute("data-translation-index")).toBe(false);
  });

  it("does not annotate <kbd> elements", () => {
    const template = buildTranslationTemplate(
      '<p>Press <kbd>Ctrl+C</kbd> to copy.</p>',
    );
    expect(template.sources.length).toBe(1);
    const container = document.createElement("div");
    container.innerHTML = template.html;
    const kbd = container.querySelector("kbd");
    expect(kbd?.hasAttribute("data-translation-index")).toBe(false);
  });

  it("does not annotate <samp> elements", () => {
    const template = buildTranslationTemplate(
      '<div><p>Output:</p><samp>Hello world</samp></div>',
    );
    // Only the <p> should be a source
    expect(template.sources).toEqual(["Output:"]);
    const container = document.createElement("div");
    container.innerHTML = template.html;
    const samp = container.querySelector("samp");
    expect(samp?.hasAttribute("data-translation-index")).toBe(false);
  });

  it("does not annotate standalone <pre> block (without code child)", () => {
    const template = buildTranslationTemplate(
      '<div><p>Intro</p><pre>raw preformatted text</pre></div>',
    );
    expect(template.sources).toEqual(["Intro"]);
    expect(template.html).toContain("raw preformatted text");
    // Ensure no translation index on pre
    const container = document.createElement("div");
    container.innerHTML = template.html;
    const pre = container.querySelector("pre");
    expect(pre?.hasAttribute("data-translation-index")).toBe(false);
  });

  it("annotates <p> normally when no code elements are present", () => {
    const template = buildTranslationTemplate(
      '<div><p>First</p><p>Second</p></div>',
    );
    expect(template.sources).toEqual(["First", "Second"]);
  });
});

describe("renderTranslatedHTML — grid/flex layout preservation", () => {
  let originalGetComputedStyle: typeof window.getComputedStyle;

  beforeEach(() => {
    originalGetComputedStyle = window.getComputedStyle;
  });

  afterEach(() => {
    window.getComputedStyle = originalGetComputedStyle;
  });

  it("wraps target and translation in a div when parent is a grid container", () => {
    // Simulate grid parent via mock getComputedStyle
    window.getComputedStyle = vi.fn((el: Element) => {
      const real = originalGetComputedStyle(el);
      // The outer container element has class "grid-parent"
      if ((el as HTMLElement).classList?.contains("grid-parent")) {
        return { ...real, display: "grid" } as CSSStyleDeclaration;
      }
      return real;
    }) as unknown as typeof window.getComputedStyle;

    const template = buildTranslationTemplate(
      '<div class="grid-parent"><p class="col-span-2">Grid item text</p></div>',
    );
    expect(template.sources).toEqual(["Grid item text"]);

    const rendered = renderTranslatedHTML(
      template.html,
      [{ index: 1, translated: "网格项译文", status: "done" }],
      false,
    );

    const container = document.createElement("div");
    container.innerHTML = rendered;

    const gridParent = container.querySelector(".grid-parent");
    // The grid parent should now have 1 child: the wrapper div
    expect(gridParent?.children.length).toBe(1);

    const wrapper = gridParent?.children[0] as HTMLElement;
    // Wrapper took over the class from the <p>
    expect(wrapper.tagName.toLowerCase()).toBe("div");
    expect(wrapper.classList.contains("col-span-2")).toBe(true);

    // The wrapper contains both the original <p> and the translation block
    expect(wrapper.children.length).toBe(2);
    expect(wrapper.querySelector(".translation-block")).not.toBeNull();
    expect(wrapper.textContent).toContain("网格项译文");
  });

  it("wraps target and translation in a div when parent is a flex container", () => {
    window.getComputedStyle = vi.fn((el: Element) => {
      const real = originalGetComputedStyle(el);
      if ((el as HTMLElement).classList?.contains("flex-parent")) {
        return { ...real, display: "flex" } as CSSStyleDeclaration;
      }
      return real;
    }) as unknown as typeof window.getComputedStyle;

    const template = buildTranslationTemplate(
      '<div class="flex-parent"><p>Flex item text</p></div>',
    );
    const rendered = renderTranslatedHTML(
      template.html,
      [{ index: 1, translated: "Flex项译文", status: "done" }],
      false,
    );

    const container = document.createElement("div");
    container.innerHTML = rendered;

    const flexParent = container.querySelector(".flex-parent");
    // Still only 1 direct child (the wrapper), not 2
    expect(flexParent?.children.length).toBe(1);
    expect(flexParent?.children[0].querySelector(".translation-block")).not.toBeNull();
  });

  it("uses insertAdjacentElement for normal block container (not grid/flex)", () => {
    // Default jsdom returns "" for display on elements without inline style,
    // so isGridOrFlexContainer will return false and we use the normal path.
    const template = buildTranslationTemplate(
      '<div><p>Normal paragraph</p></div>',
    );
    const rendered = renderTranslatedHTML(
      template.html,
      [{ index: 1, translated: "普通段落译文", status: "done" }],
      false,
    );

    const container = document.createElement("div");
    container.innerHTML = rendered;

    const outer = container.firstElementChild;
    // 2 children: the original <p> and the translation block sibling
    expect(outer?.children.length).toBe(2);
    expect(outer?.children[1].classList.contains("translation-block")).toBe(true);
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

  it("appends translation inside table cells instead of as sibling to preserve table structure", () => {
    const template = buildTranslationTemplate(
      '<table><tr><td>Cell one</td><td>Cell two</td></tr></table>',
    );

    const rendered = renderTranslatedHTML(
      template.html,
      [
        { index: 1, translated: "单元格一", status: "done" },
        { index: 2, translated: "单元格二", status: "done" },
      ],
      false,
    );

    const container = document.createElement("div");
    container.innerHTML = rendered;

    // Translation blocks must be children of <td>, not siblings
    const tds = container.querySelectorAll("td");
    expect(tds.length).toBe(2);
    for (const td of tds) {
      expect(td.querySelector(".translation-block")).not.toBeNull();
    }

    // The <tr> must still have exactly 2 children (the two <td>s)
    const tr = container.querySelector("tr");
    expect(tr?.children.length).toBe(2);

    expect(rendered).toContain("单元格一");
    expect(rendered).toContain("单元格二");
  });

  it("preserves thead/tbody multi-row table structure with th and td translations", () => {
    const html = [
      "<table><thead><tr><th>Name</th><th>Score</th></tr></thead>",
      "<tbody><tr><td>Alice</td><td>95</td></tr>",
      "<tr><td>Bob</td><td>87</td></tr></tbody></table>",
    ].join("");
    const template = buildTranslationTemplate(html);

    // Backend returns translations for all 6 cells
    const paragraphs = template.sources.map((_, i) => ({
      index: i + 1,
      translated: `翻译${i + 1}`,
      status: "done" as const,
    }));

    const rendered = renderTranslatedHTML(template.html, paragraphs, false);
    const container = document.createElement("div");
    container.innerHTML = rendered;

    // Table element count must be unchanged
    const ths = container.querySelectorAll("th");
    const tds = container.querySelectorAll("td");
    expect(ths.length).toBe(2);
    expect(tds.length).toBe(4);

    // Every th/td must contain a .translation-block child
    for (const cell of [...ths, ...tds]) {
      expect(cell.querySelector(".translation-block")).not.toBeNull();
    }

    // Each <tr> child count must equal original column count (no extra siblings)
    for (const tr of container.querySelectorAll("tr")) {
      expect(tr.children.length).toBe(2);
    }
  });

  it("places pending indicators inside table cells during translation", () => {
    const template = buildTranslationTemplate(
      "<table><tr><td>Pending cell</td></tr></table>",
    );

    const rendered = renderTranslatedHTML(template.html, [], true);
    const container = document.createElement("div");
    container.innerHTML = rendered;

    const td = container.querySelector("td");
    expect(td?.querySelector(".immersive-translation-pending")).not.toBeNull();
    // <tr> still has exactly 1 child
    expect(td?.parentElement?.children.length).toBe(1);
  });

  it("handles mixed table and paragraph content correctly", () => {
    const html = "<p>Intro text</p><table><tr><td>Cell</td></tr></table><p>Outro text</p>";
    const template = buildTranslationTemplate(html);

    const paragraphs = template.sources.map((_, i) => ({
      index: i + 1,
      translated: `译文${i + 1}`,
      status: "done" as const,
    }));

    const rendered = renderTranslatedHTML(template.html, paragraphs, false);
    const container = document.createElement("div");
    container.innerHTML = rendered;

    // <p> translations are siblings (afterend), <td> translation is a child
    const ps = container.querySelectorAll("p");
    for (const p of ps) {
      const next = p.nextElementSibling;
      expect(next?.classList.contains("immersive-translation")).toBe(true);
    }

    const td = container.querySelector("td");
    expect(td?.querySelector(".translation-block")).not.toBeNull();
    expect(td?.parentElement?.children.length).toBe(1);
  });
});
