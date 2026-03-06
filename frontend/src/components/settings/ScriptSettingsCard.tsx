import type { ChangeEvent } from "react";
import type { Feed } from "@/types";
import type { ScriptLang } from "./types";

type ScriptSettingsCardProps = {
  scriptFeedID: number | null;
  feeds: Feed[];
  scriptLang: ScriptLang;
  scriptContent: string;
  onSelectScriptFeed: (feedID: number | null) => void;
  onUploadScriptFile: (event: ChangeEvent<HTMLInputElement>) => void;
  onScriptLangChange: (lang: ScriptLang) => void;
  onScriptContentChange: (value: string) => void;
  onSaveFeedScript: () => void;
};

export function ScriptSettingsCard({
  scriptFeedID,
  feeds,
  scriptLang,
  scriptContent,
  onSelectScriptFeed,
  onUploadScriptFile,
  onScriptLangChange,
  onScriptContentChange,
  onSaveFeedScript,
}: ScriptSettingsCardProps) {
  return (
    <div className="settings-page-inner settings-section-card">
      <h4 className="section-title">脚本设置（按订阅源）</h4>
      <label htmlFor="scriptFeed">订阅源</label>
      <select id="scriptFeed" value={scriptFeedID ?? ""} onChange={(e) => onSelectScriptFeed(e.target.value ? Number(e.target.value) : null)}>
        <option value="">请选择</option>
        {feeds.map((feed) => (
          <option key={feed.id} value={feed.id}>
            {feed.title || feed.url}
          </option>
        ))}
      </select>
      <label htmlFor="scriptUpload">上传脚本文件</label>
      <input id="scriptUpload" type="file" accept=".sh,.txt,.js,.py,.rb,.pl,.bash" onChange={onUploadScriptFile} />
      <label htmlFor="scriptLang">脚本语言</label>
      <select id="scriptLang" value={scriptLang} onChange={(e) => onScriptLangChange(e.target.value as ScriptLang)}>
        <option value="shell">shell</option>
        <option value="python">python</option>
        <option value="javascript">javascript</option>
      </select>
      <label htmlFor="scriptContent">脚本内容（stdin 为 JSON v1，stdout 必须返回 JSON，content_html 为最终全文）</label>
      <textarea
        id="scriptContent"
        className="script-editor"
        rows={8}
        value={scriptContent}
        onChange={(e) => onScriptContentChange(e.target.value)}
        placeholder={`#!/bin/sh\n# stdin 为 JSON v1，stdout 返回 JSON（示例）\necho '{"ok":true,"content_html":"<article>...</article>"}'`}
      />
      <div className="row">
        <button className="secondary" onClick={onSaveFeedScript}>
          保存脚本
        </button>
      </div>
    </div>
  );
}
