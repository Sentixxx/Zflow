import { useState, useEffect, useMemo, useCallback } from "react";
import { useLocation } from "wouter";
import { ApiClient } from "@/api";
import type { TopicBrief } from "@/types";
import { cn } from "@/lib/utils";
import { resolveInitialAPIBase } from "@/lib/api-base";

type BriefLevel = "daily" | "weekly" | "monthly";

const LEVEL_LABELS: Record<BriefLevel, string> = {
  daily: "日报",
  weekly: "周报",
  monthly: "月报",
};

export function BriefsPage() {
  const [, navigate] = useLocation();
  const [level, setLevel] = useState<BriefLevel>("daily");
  const [briefs, setBriefs] = useState<TopicBrief[]>([]);
  const [selectedBrief, setSelectedBrief] = useState<TopicBrief | null>(null);
  const [loading, setLoading] = useState(false);

  const api = useMemo(
    () => new ApiClient(resolveInitialAPIBase(localStorage.getItem("zflow_api_base"), window.location.hostname)),
    [],
  );

  const fetchBriefs = useCallback(async (l: BriefLevel) => {
    setLoading(true);
    try {
      const data = await api.listBriefs(l, 30);
      setBriefs(data);
      setSelectedBrief(data[0] ?? null);
    } catch {
      setBriefs([]);
      setSelectedBrief(null);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    fetchBriefs(level);
  }, [level, fetchBriefs]);

  return (
    <div className="flex flex-col h-screen bg-background text-foreground">
      {/* Header */}
      <header className="flex items-center justify-between px-6 py-3 border-b shrink-0" role="banner">
        <div className="flex items-center gap-4">
          <button
            onClick={() => navigate("/")}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          >
            ← 返回阅读器
          </button>
          <h1 className="text-lg font-semibold">知识简报</h1>
        </div>
        <div className="flex gap-1 bg-muted rounded-lg p-0.5">
          {(["daily", "weekly", "monthly"] as BriefLevel[]).map((l) => (
            <button
              key={l}
              onClick={() => setLevel(l)}
              aria-current={level === l ? "true" : undefined}
              className={cn(
                "px-3 py-1.5 text-sm rounded-md transition-colors cursor-pointer focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
                level === l
                  ? "bg-background text-foreground font-medium shadow-sm"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {LEVEL_LABELS[l]}
            </button>
          ))}
        </div>
      </header>

      {/* Content */}
      <div className="flex flex-1 min-h-0">
        {/* Brief list */}
        <aside className="w-72 shrink-0 border-r overflow-y-auto">
          {loading && (
            <div className="p-2 space-y-1">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="px-4 py-3 border-b animate-pulse space-y-2">
                  <div className="h-4 bg-muted rounded w-3/4" />
                  <div className="h-3 bg-muted rounded w-1/2" />
                  <div className="h-3 bg-muted rounded w-1/3" />
                </div>
              ))}
            </div>
          )}
          {!loading && briefs.length === 0 && (
            <div className="p-4 text-sm text-muted-foreground">
              暂无{LEVEL_LABELS[level]}。Agent 运行后将自动生成。
            </div>
          )}
          {briefs.map((brief) => (
            <button
              key={brief.id}
              onClick={() => setSelectedBrief(brief)}
              className={cn(
                "w-full text-left px-4 py-3 border-b transition-colors cursor-pointer focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
                selectedBrief?.id === brief.id
                  ? "bg-accent"
                  : "hover:bg-muted/50",
              )}
            >
              <div className="text-sm font-medium line-clamp-2">{brief.title}</div>
              <div className="text-xs text-muted-foreground mt-1">
                {brief.period_start}
                {brief.period_end !== brief.period_start && ` ~ ${brief.period_end}`}
              </div>
              <div className="text-xs text-muted-foreground">
                {brief.source_article_ids?.length ?? 0} 篇文章
              </div>
            </button>
          ))}
        </aside>

        {/* Brief detail */}
        <main className="flex-1 min-w-0 overflow-y-auto p-6">
          {selectedBrief ? (
            <article className="max-w-3xl mx-auto">
              <h2 className="text-2xl font-bold mb-2">{selectedBrief.title}</h2>
              <div className="text-sm text-muted-foreground mb-6">
                {selectedBrief.period_start}
                {selectedBrief.period_end !== selectedBrief.period_start && ` ~ ${selectedBrief.period_end}`}
                {" · "}
                {LEVEL_LABELS[selectedBrief.level]}
                {" · "}
                {selectedBrief.source_article_ids?.length ?? 0} 篇源文章
              </div>
              <div
                className="prose prose-sm dark:prose-invert max-w-none"
                dangerouslySetInnerHTML={{ __html: renderMarkdown(selectedBrief.content) }}
              />
            </article>
          ) : (
            <div className="flex items-center justify-center h-full text-muted-foreground">
              {loading ? (
                <div className="w-full max-w-3xl mx-auto space-y-4 animate-pulse">
                  <div className="h-7 bg-muted rounded w-2/3" />
                  <div className="h-4 bg-muted rounded w-1/3" />
                  <div className="space-y-2 mt-6">
                    <div className="h-4 bg-muted rounded w-full" />
                    <div className="h-4 bg-muted rounded w-5/6" />
                    <div className="h-4 bg-muted rounded w-4/6" />
                  </div>
                </div>
              ) : "选择一篇简报查看详情"}
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

function renderMarkdown(md: string): string {
  // Simple markdown to HTML: headers, bold, lists, paragraphs
  return md
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/^### (.+)$/gm, "<h3>$1</h3>")
    .replace(/^## (.+)$/gm, "<h2>$1</h2>")
    .replace(/^# (.+)$/gm, "<h1>$1</h1>")
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/^- (.+)$/gm, "<li>$1</li>")
    .replace(/(<li>.*<\/li>\n?)+/g, "<ul>$&</ul>")
    .replace(/\n\n/g, "</p><p>")
    .replace(/^(?!<[hul])/, "<p>")
    .replace(/(?<![>])$/, "</p>");
}
