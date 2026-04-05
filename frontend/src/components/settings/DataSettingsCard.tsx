import type { ChangeEvent } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";

type DataSettingsCardProps = {
  articleRetentionDays: string;
  selectedArticleID: number | null;
  selectedArticleTitle: string;
  isRefreshingCurrentArticleAISummary: boolean;
  onArticleRetentionDaysChange: (value: string) => void;
  onSaveDataSettings: () => void;
  onRefreshCurrentArticleAISummary: () => void;
  onClearCurrentArticleAISummary: () => void;
  onClearRecentAISummaries: () => void;
  onRegenerateSummaries: () => void;
  onExportProfileJSON: () => void;
  onExportOPML: () => void;
  onImportProfileJSON: (event: ChangeEvent<HTMLInputElement>) => void;
  onImportOPML: (event: ChangeEvent<HTMLInputElement>) => void;
};

export function DataSettingsCard({
  articleRetentionDays,
  selectedArticleID,
  selectedArticleTitle,
  isRefreshingCurrentArticleAISummary,
  onArticleRetentionDaysChange,
  onSaveDataSettings,
  onRefreshCurrentArticleAISummary,
  onClearCurrentArticleAISummary,
  onClearRecentAISummaries,
  onRegenerateSummaries,
  onExportProfileJSON,
  onExportOPML,
  onImportProfileJSON,
  onImportOPML,
}: DataSettingsCardProps) {
  return (
    <div className="space-y-6">
      {/* 清理策略 */}
      <div className="space-y-3">
        <h4 className="text-base font-semibold">清理策略</h4>
        <div className="space-y-1.5">
          <Label htmlFor="retentionDays">文章保留天数（收藏不会被清理）</Label>
          <div className="flex gap-2">
            <Input
              id="retentionDays"
              type="number"
              min={1}
              max={3650}
              value={articleRetentionDays}
              onChange={(e) => onArticleRetentionDaysChange(e.target.value)}
              className="w-32"
            />
            <Button variant="outline" onClick={onSaveDataSettings}>保存策略</Button>
          </div>
        </div>
      </div>

      <Separator />

      {/* 摘要维护 */}
      <div className="space-y-3">
        <h4 className="text-base font-semibold">摘要维护</h4>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={onRegenerateSummaries}>
            重生成最近 100 篇摘要
          </Button>
        </div>
      </div>

      <Separator />

      {/* 开发调试 */}
      <div className="space-y-3">
        <h4 className="text-base font-semibold">开发调试</h4>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={onRefreshCurrentArticleAISummary}
            disabled={selectedArticleID == null || isRefreshingCurrentArticleAISummary}
          >
            {isRefreshingCurrentArticleAISummary ? "正在刷新当前文章摘要..." : "刷新当前文章摘要"}
          </Button>
          <Button
            variant="outline"
            onClick={onClearCurrentArticleAISummary}
            disabled={selectedArticleID == null}
          >
            清除当前文章 AI 摘要
          </Button>
          <Button variant="outline" onClick={onClearRecentAISummaries}>
            清除最近 100 篇 AI 摘要
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          {selectedArticleID == null
            ? "当前未选中文章；单篇刷新/清除按钮已禁用。"
            : `${isRefreshingCurrentArticleAISummary ? "当前文章摘要刷新中" : "当前文章"}：${selectedArticleTitle || `#${selectedArticleID}`}`}
        </p>
      </div>

      <Separator />

      {/* 数据导出 */}
      <div className="space-y-3">
        <h4 className="text-base font-semibold">数据导出</h4>
        <div className="flex flex-wrap gap-2">
          <Button onClick={onExportProfileJSON}>导出个人配置（JSON）</Button>
          <Button variant="outline" onClick={onExportOPML}>导出订阅源（OPML）</Button>
        </div>
      </div>

      <Separator />

      {/* 数据导入 */}
      <div className="space-y-4">
        <h4 className="text-base font-semibold">数据导入</h4>
        <div className="space-y-1.5">
          <Label htmlFor="importProfile">导入个人配置（JSON，含分类与脚本）</Label>
          <input
            id="importProfile"
            type="file"
            accept=".json,application/json"
            onChange={onImportProfileJSON}
            className="block w-full text-sm text-muted-foreground file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-input file:text-sm file:bg-background file:text-foreground hover:file:bg-muted/60 cursor-pointer"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="importOPML">导入订阅源（OPML）</Label>
          <input
            id="importOPML"
            type="file"
            accept=".opml,.xml,text/xml,application/xml"
            onChange={onImportOPML}
            className="block w-full text-sm text-muted-foreground file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-input file:text-sm file:bg-background file:text-foreground hover:file:bg-muted/60 cursor-pointer"
          />
        </div>
      </div>
    </div>
  );
}
