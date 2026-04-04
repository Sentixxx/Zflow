import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => {
  vi.restoreAllMocks();
});

function createStream(chunks: string[]) {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      chunks.forEach((chunk) => controller.enqueue(encoder.encode(chunk)));
      controller.close();
    },
  });
}

describe("translateArticleStream", () => {
  it("skips invalid stream lines instead of throwing", async () => {
    const client = new ApiClient("http://example.com");
    const onEvent = vi.fn();
    const stream = createStream([
      '{"type":"start","article_id":1,"target_lang":"zh-CN","total":1}\n',
      "bad-json\n",
    ]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(stream, { status: 200 }));

    await expect(client.translateArticleStream(1, "zh-CN", onEvent)).resolves.toBeUndefined();
    expect(onEvent).toHaveBeenCalledTimes(1);
  });
});

describe("listArticlesPage", () => {
  it("uses limit+1 to detect hasMore", async () => {
    const client = new ApiClient("http://example.com");
    const articles = Array.from({ length: 3 }, (_, idx) => ({ id: idx + 1 })) as unknown[];
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ articles, has_more: false }), { status: 200 }),
    );

    const result = await client.listArticlesPage(1, 2);
    expect(result.articles).toHaveLength(2);
    expect(result.hasMore).toBe(true);
  });
});
