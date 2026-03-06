import type { ChangeEvent } from "react";

type DataSettingsCardProps = {
  articleRetentionDays: string;
  onArticleRetentionDaysChange: (value: string) => void;
  onSaveDataSettings: () => void;
  onExportProfileJSON: () => void;
  onExportOPML: () => void;
  onImportProfileJSON: (event: ChangeEvent<HTMLInputElement>) => void;
  onImportOPML: (event: ChangeEvent<HTMLInputElement>) => void;
};

export function DataSettingsCard({
  articleRetentionDays,
  onArticleRetentionDaysChange,
  onSaveDataSettings,
  onExportProfileJSON,
  onExportOPML,
  onImportProfileJSON,
  onImportOPML,
}: DataSettingsCardProps) {
  return (
    <div className="settings-page-inner settings-section-card">
      <h4 className="section-title">清理策略</h4>
      <label htmlFor="retentionDays">文章保留天数（收藏不会被清理）</label>
      <div className="row">
        <input id="retentionDays" type="number" min={1} max={3650} value={articleRetentionDays} onChange={(e) => onArticleRetentionDaysChange(e.target.value)} />
        <button className="secondary" onClick={onSaveDataSettings}>
          保存策略
        </button>
      </div>

      <h4 className="section-title">数据导出</h4>
      <div className="row settings-actions">
        <button onClick={onExportProfileJSON}>导出个人配置（JSON）</button>
        <button className="secondary" onClick={onExportOPML}>
          导出订阅源（OPML）
        </button>
      </div>

      <h4 className="section-title">数据导入</h4>
      <label htmlFor="importProfile">导入个人配置（JSON，含分类与脚本）</label>
      <input id="importProfile" type="file" accept=".json,application/json" onChange={onImportProfileJSON} />

      <label htmlFor="importOPML">导入订阅源（OPML）</label>
      <input id="importOPML" type="file" accept=".opml,.xml,text/xml,application/xml" onChange={onImportOPML} />
    </div>
  );
}
