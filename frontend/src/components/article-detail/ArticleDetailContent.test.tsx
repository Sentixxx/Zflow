/* @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { ArticleDetailContent } from "./ArticleDetailContent";
import type { Article } from "@/types";
import { TooltipProvider } from "@/components/ui/tooltip";
import { buildTranslationTemplate } from "@/lib/translation";

function makeArticle(): Article {
  return {
    id: 1,
    feed_id: 1,
    title: "Translated Article",
    link: "https://example.com/post",
    full_content: "<p>Original body</p>",
    display_summary: "Summary",
    created_at: "2026-04-06T00:00:00Z",
    is_read: false,
    is_favorite: false,
  };
}

describe("ArticleDetailContent", () => {
  it("falls back to original body when translation mode is turned off", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);

    act(() => {
      root.render(
        <TooltipProvider>
          <ArticleDetailContent
            article={makeArticle()}
            sanitizedSummaryHTML="<p>Summary</p>"
            sanitizedFullContentHTML="<p>Original body</p>"
            translationTemplateHTML={buildTranslationTemplate("<p>Original body</p>").html}
            canMarkUnread={false}
            canToggleFavorite={false}
            isFavorite={false}
            canOpenSourceSite={false}
            canExtractReadable={false}
            isExtractingReadable={false}
            canRefreshArticleCache={false}
            isRefreshingArticleCache={false}
            isTranslatingArticle={false}
            isTranslationVisible={false}
            sourceSiteURL=""
            detailProgressText=""
            canGoPrev={false}
            canGoNext={false}
            translationParagraphs={[
              { index: 1, source: "Source paragraph", translated: "Translated paragraph", status: "done" },
            ]}
            onMarkUnread={() => undefined}
            onToggleFavorite={() => undefined}
            onOpenSourceSite={() => undefined}
            onExtractReadable={() => undefined}
            onRefreshArticleCache={() => undefined}
            onTranslateArticle={() => undefined}
            onGoPrev={() => undefined}
            onGoNext={() => undefined}
          />
        </TooltipProvider>,
      );
    });

    expect(container.textContent).toContain("Original body");
    expect(container.textContent).not.toContain("Translated paragraph");
  });

  it("keeps showing original body while translation is still running but translation view is hidden", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);

    act(() => {
      root.render(
        <TooltipProvider>
          <ArticleDetailContent
            article={makeArticle()}
            sanitizedSummaryHTML="<p>Summary</p>"
            sanitizedFullContentHTML="<p>Original body</p>"
            translationTemplateHTML={buildTranslationTemplate("<p>Original body</p>").html}
            canMarkUnread={false}
            canToggleFavorite={false}
            isFavorite={false}
            canOpenSourceSite={false}
            canExtractReadable={false}
            isExtractingReadable={false}
            canRefreshArticleCache={false}
            isRefreshingArticleCache={false}
            isTranslatingArticle={true}
            isTranslationVisible={false}
            sourceSiteURL=""
            detailProgressText=""
            canGoPrev={false}
            canGoNext={false}
            translationParagraphs={[
              { index: 1, source: "Source paragraph", translated: "Translated paragraph", status: "pending" },
            ]}
            onMarkUnread={() => undefined}
            onToggleFavorite={() => undefined}
            onOpenSourceSite={() => undefined}
            onExtractReadable={() => undefined}
            onRefreshArticleCache={() => undefined}
            onTranslateArticle={() => undefined}
            onGoPrev={() => undefined}
            onGoNext={() => undefined}
          />
        </TooltipProvider>,
      );
    });

    expect(container.textContent).toContain("Original body");
    expect(container.textContent).not.toContain("Source paragraph");
    expect(container.textContent).not.toContain("翻译中");
  });

  it("keeps original html blocks and appends translation below matching paragraphs", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    const html = '<div class="article-body"><p>First paragraph</p><figure class="hero"><img src="/cover.png" alt="cover"></figure><p>Second paragraph</p></div>';

    act(() => {
      root.render(
        <TooltipProvider>
          <ArticleDetailContent
            article={{ ...makeArticle(), full_content: html }}
            sanitizedSummaryHTML="<p>Summary</p>"
            sanitizedFullContentHTML={html}
            translationTemplateHTML={buildTranslationTemplate(html).html}
            canMarkUnread={false}
            canToggleFavorite={false}
            isFavorite={false}
            canOpenSourceSite={false}
            canExtractReadable={false}
            isExtractingReadable={false}
            canRefreshArticleCache={false}
            isRefreshingArticleCache={false}
            isTranslatingArticle={false}
            isTranslationVisible={true}
            sourceSiteURL=""
            detailProgressText=""
            canGoPrev={false}
            canGoNext={false}
            translationParagraphs={[
              { index: 1, source: "First paragraph", translated: "第一段译文", status: "done" },
              { index: 2, source: "Second paragraph", translated: "第二段译文", status: "done" },
            ]}
            onMarkUnread={() => undefined}
            onToggleFavorite={() => undefined}
            onOpenSourceSite={() => undefined}
            onExtractReadable={() => undefined}
            onRefreshArticleCache={() => undefined}
            onTranslateArticle={() => undefined}
            onGoPrev={() => undefined}
            onGoNext={() => undefined}
          />
        </TooltipProvider>,
      );
    });

    const image = container.querySelector('img[alt="cover"]');
    const wrapper = container.querySelector(".article-body");
    expect(image).not.toBeNull();
    expect(wrapper).not.toBeNull();
    expect(container.textContent).toContain("First paragraph");
    expect(container.textContent).toContain("第一段译文");
    expect(container.textContent).toContain("Second paragraph");
    expect(container.textContent).toContain("第二段译文");
  });

  it("renders source appendix at the bottom without replacing readable content", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);

    act(() => {
      root.render(
        <TooltipProvider>
          <ArticleDetailContent
            article={{
              ...makeArticle(),
              summary: "<p>Original RSS summary</p>",
              source_payload: {
                title: "Original RSS title",
                link: "https://news.ycombinator.com/item?id=1",
                summary: "<p>Original RSS summary</p>",
                published_at: "2026-04-07T00:00:00Z",
                fields: [
                  { key: "comments", value: "https://news.ycombinator.com/item?id=1" },
                  { key: "encoded", value_html: "<p>Encoded body</p>" },
                ],
              },
            }}
            sanitizedSummaryHTML="<p>Summary</p>"
            sanitizedFullContentHTML="<p>Readable body</p>"
            translationTemplateHTML={buildTranslationTemplate("<p>Readable body</p>").html}
            canMarkUnread={false}
            canToggleFavorite={false}
            isFavorite={false}
            canOpenSourceSite={false}
            canExtractReadable={false}
            isExtractingReadable={false}
            canRefreshArticleCache={false}
            isRefreshingArticleCache={false}
            isTranslatingArticle={false}
            isTranslationVisible={false}
            sourceSiteURL=""
            detailProgressText=""
            canGoPrev={false}
            canGoNext={false}
            translationParagraphs={[]}
            onMarkUnread={() => undefined}
            onToggleFavorite={() => undefined}
            onOpenSourceSite={() => undefined}
            onExtractReadable={() => undefined}
            onRefreshArticleCache={() => undefined}
            onTranslateArticle={() => undefined}
            onGoPrev={() => undefined}
            onGoNext={() => undefined}
          />
        </TooltipProvider>,
      );
    });

    expect(container.textContent).toContain("Readable body");
    expect(container.textContent).toContain("原始条目附录");
    expect(container.textContent).toContain("Original RSS title");
    expect(container.textContent).toContain("https://news.ycombinator.com/item?id=1");
    expect(container.textContent).toContain("Encoded body");
  });

  it("falls back to article core fields when source payload is missing", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);

    act(() => {
      root.render(
        <TooltipProvider>
          <ArticleDetailContent
            article={{
              ...makeArticle(),
              title: "Music for Programming",
              link: "https://news.ycombinator.com/item?id=123",
              summary: "<p>HN comment thread</p>",
              published_at: "2026-04-07T00:00:00Z",
              source_payload: undefined,
            }}
            sanitizedSummaryHTML=""
            sanitizedFullContentHTML=""
            translationTemplateHTML=""
            canMarkUnread={false}
            canToggleFavorite={false}
            isFavorite={false}
            canOpenSourceSite={false}
            canExtractReadable={false}
            isExtractingReadable={false}
            canRefreshArticleCache={false}
            isRefreshingArticleCache={false}
            isTranslatingArticle={false}
            isTranslationVisible={false}
            sourceSiteURL=""
            detailProgressText=""
            canGoPrev={false}
            canGoNext={false}
            translationParagraphs={[]}
            onMarkUnread={() => undefined}
            onToggleFavorite={() => undefined}
            onOpenSourceSite={() => undefined}
            onExtractReadable={() => undefined}
            onRefreshArticleCache={() => undefined}
            onTranslateArticle={() => undefined}
            onGoPrev={() => undefined}
            onGoNext={() => undefined}
          />
        </TooltipProvider>,
      );
    });

    expect(container.textContent).toContain("原始条目附录");
    expect(container.textContent).toContain("Music for Programming");
    expect(container.textContent).toContain("https://news.ycombinator.com/item?id=123");
    expect(container.textContent).toContain("HN comment thread");
  });
});
