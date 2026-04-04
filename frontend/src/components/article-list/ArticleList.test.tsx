/* @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { ArticleList } from "./ArticleList";
import type { Article, Feed } from "@/types";

function makeArticle(id: number): Article {
  return {
    id,
    feed_id: 1,
    title: `Article ${id}`,
    link: "https://example.com",
    created_at: "2026-04-01T00:00:00Z",
    is_read: false,
    is_favorite: false,
  };
}

function makeFeed(): Feed {
  return {
    id: 1,
    url: "https://example.com/feed.xml",
    title: "Example Feed",
    item_count: 1,
    last_fetched_at: "2026-04-01T00:00:00Z",
    last_fetch_status: "ok",
    created_at: "2026-04-01T00:00:00Z",
  };
}

describe("ArticleList", () => {
  it("marks list as virtualized when rendering articles", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    const feed = makeFeed();
    const feedByID = new Map<number, Feed>([[feed.id, feed]]);
    const feedNameByID = new Map<number, string>([[feed.id, feed.title]]);

    act(() => {
      root.render(
        <ArticleList
          articles={[makeArticle(1)]}
          selectedArticleID={null}
          feedByID={feedByID}
          feedNameByID={feedNameByID}
          apiBase="http://localhost"
          onSelectArticle={() => undefined}
          listBounce={false}
          onScroll={() => undefined}
        />,
      );
    });

    const element = container.querySelector('[data-virtualized="true"]');
    expect(element).not.toBeNull();
  });
});
