import { useState, useEffect, useMemo, useCallback } from "react";
import { ApiClient } from "@/api";
import type { AgentRun } from "@/types";
import { resolveInitialAPIBase } from "@/lib/api-base";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const AGENT_TYPES = [
  { type: "interest_profile", label: "兴趣画像" },
  { type: "topic_cluster", label: "主题聚合" },
  { type: "topic_brief_daily", label: "日报生成" },
  { type: "topic_brief_weekly", label: "周报生成" },
  { type: "topic_brief_monthly", label: "月报生成" },
];

export function AgentSettingsCard() {
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [triggering, setTriggering] = useState<string | null>(null);

  const api = useMemo(
    () => new ApiClient(resolveInitialAPIBase(localStorage.getItem("zflow_api_base"), window.location.hostname)),
    [],
  );

  const fetchRuns = useCallback(async () => {
    setLoading(true);
    try {
      setRuns(await api.listAgentRuns(undefined, 30));
    } catch {
      setRuns([]);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    fetchRuns();
  }, [fetchRuns]);

  const handleTrigger = async (agentType: string) => {
    setTriggering(agentType);
    try {
      await api.triggerAgent(agentType);
      // Wait a moment for the run to be created
      setTimeout(fetchRuns, 1500);
    } catch {
      // ignore
    } finally {
      setTriggering(null);
    }
  };

  return (
    <div className="space-y-6">
      {/* Trigger buttons */}
      <div>
        <h3 className="text-base font-semibold">手动触发 Agent</h3>
        <p className="text-sm text-muted-foreground mt-1 mb-3">
          Agent 通常自动运行，也可以手动触发。
        </p>
        <div className="flex flex-wrap gap-2">
          {AGENT_TYPES.map(({ type, label }) => (
            <Button
              key={type}
              size="sm"
              variant="outline"
              disabled={triggering !== null}
              onClick={() => handleTrigger(type)}
            >
              {triggering === type ? "触发中..." : `运行 ${label}`}
            </Button>
          ))}
        </div>
      </div>

      {/* Run history */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-base font-semibold">运行历史</h3>
          <Button size="sm" variant="ghost" onClick={fetchRuns} disabled={loading}>
            刷新
          </Button>
        </div>

        {loading && <p className="text-sm text-muted-foreground">加载中...</p>}

        {!loading && runs.length === 0 && (
          <p className="text-sm text-muted-foreground">暂无运行记录。</p>
        )}

        {runs.length > 0 && (
          <div className="space-y-1.5 max-h-96 overflow-y-auto">
            {runs.map((run) => (
              <div key={run.id} className="flex items-center gap-3 px-3 py-2 rounded-md border text-sm">
                <Badge
                  variant={run.status === "completed" ? "default" : run.status === "failed" ? "destructive" : "secondary"}
                  className={cn("shrink-0", run.status === "completed" && "bg-green-600")}
                >
                  {run.status === "completed" ? "完成" : run.status === "failed" ? "失败" : "运行中"}
                </Badge>
                <span className="font-medium shrink-0">{run.agent_type}</span>
                <span className="text-muted-foreground truncate flex-1">
                  {run.error || run.output_summary || ""}
                </span>
                <span className="text-xs text-muted-foreground shrink-0">
                  {new Date(run.started_at).toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
