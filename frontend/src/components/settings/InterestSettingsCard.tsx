import { useState, useEffect, useMemo, useCallback } from "react";
import { ApiClient } from "@/api";
import type { InterestProfile } from "@/types";
import { resolveInitialAPIBase } from "@/lib/api-base";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function InterestSettingsCard() {
  const [profiles, setProfiles] = useState<InterestProfile[]>([]);
  const [loading, setLoading] = useState(false);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [editWeight, setEditWeight] = useState("");

  const api = useMemo(
    () => new ApiClient(resolveInitialAPIBase(localStorage.getItem("zflow_api_base"), window.location.hostname)),
    [],
  );

  const fetchProfiles = useCallback(async () => {
    setLoading(true);
    try {
      setProfiles(await api.listInterests());
    } catch {
      setProfiles([]);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    fetchProfiles();
  }, [fetchProfiles]);

  const handleUpdateWeight = async (id: number) => {
    const weight = parseFloat(editWeight);
    if (isNaN(weight) || weight < 0) return;
    try {
      await api.updateInterestWeight(id, weight);
      setEditingID(null);
      fetchProfiles();
    } catch {
      // ignore
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteInterest(id);
      fetchProfiles();
    } catch {
      // ignore
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-base font-semibold">兴趣画像</h3>
        <p className="text-sm text-muted-foreground mt-1">
          基于阅读和收藏行为自动提炼的兴趣向量，用于提升文章相关性评分。
        </p>
      </div>

      {loading && <p className="text-sm text-muted-foreground">加载中...</p>}

      {!loading && profiles.length === 0 && (
        <p className="text-sm text-muted-foreground">
          暂无兴趣画像。阅读和收藏文章后，Agent 将自动生成。
        </p>
      )}

      {profiles.length > 0 && (
        <div className="space-y-2">
          {profiles.map((p) => (
            <div key={p.id} className="flex items-center gap-3 p-3 rounded-lg border bg-card">
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate">{p.label}</div>
                <div className="text-xs text-muted-foreground mt-0.5">
                  权重: {p.weight.toFixed(2)} · {p.source_article_ids?.length ?? 0} 篇来源文章
                  {p.last_reinforced_at && (
                    <> · 最近强化: {new Date(p.last_reinforced_at).toLocaleDateString()}</>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                {editingID === p.id ? (
                  <>
                    <Input
                      type="number"
                      step="0.1"
                      min="0"
                      value={editWeight}
                      onChange={(e) => setEditWeight(e.target.value)}
                      className="w-20 h-8 text-sm"
                    />
                    <Button size="sm" variant="outline" onClick={() => handleUpdateWeight(p.id)}>
                      保存
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setEditingID(null)}>
                      取消
                    </Button>
                  </>
                ) : (
                  <>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setEditingID(p.id);
                        setEditWeight(String(p.weight));
                      }}
                    >
                      调整权重
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => handleDelete(p.id)}
                    >
                      删除
                    </Button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
