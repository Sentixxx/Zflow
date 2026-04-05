import type { ChangeEvent } from "react";
import type { Feed } from "@/types";
import type { ScriptLang } from "./types";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

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
    <div className="space-y-6">
      <div>
        <h4 className="text-base font-semibold mb-4">脚本设置（按订阅源）</h4>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="scriptFeed">订阅源</Label>
            <select
              id="scriptFeed"
              value={scriptFeedID ?? ""}
              onChange={(e) => onSelectScriptFeed(e.target.value ? Number(e.target.value) : null)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="">请选择</option>
              {feeds.map((feed) => (
                <option key={feed.id} value={feed.id}>
                  {feed.title || feed.url}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="scriptUpload">上传脚本文件</Label>
            <input
              id="scriptUpload"
              type="file"
              accept=".sh,.txt,.js,.py,.rb,.pl,.bash"
              onChange={onUploadScriptFile}
              className="block w-full text-sm text-muted-foreground file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border file:border-input file:text-sm file:bg-background file:text-foreground hover:file:bg-muted/60 cursor-pointer"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="scriptLang">脚本语言</Label>
            <select
              id="scriptLang"
              value={scriptLang}
              onChange={(e) => onScriptLangChange(e.target.value as ScriptLang)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="shell">shell</option>
              <option value="python">python</option>
              <option value="javascript">javascript</option>
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="scriptContent">
              脚本内容（stdin 为 JSON v1，stdout 必须返回 JSON，content_html 为最终全文）
            </Label>
            <Textarea
              id="scriptContent"
              rows={8}
              value={scriptContent}
              onChange={(e) => onScriptContentChange(e.target.value)}
              placeholder={`#!/bin/sh\n# stdin 为 JSON v1，stdout 返回 JSON（示例）\necho '{"ok":true,"content_html":"<article>...</article>"}'`}
              className="font-mono text-xs"
            />
          </div>

          <div>
            <Button variant="outline" onClick={onSaveFeedScript}>保存脚本</Button>
          </div>
        </div>
      </div>
    </div>
  );
}
