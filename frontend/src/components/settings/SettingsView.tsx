import type { ChangeEvent } from "react";
import type { Feed, Folder } from "@/types";
import { SubscriptionSettingsCard } from "./SubscriptionSettingsCard";
import { ScriptSettingsCard } from "./ScriptSettingsCard";
import { ConnectionSettingsCard } from "./ConnectionSettingsCard";
import { AISettingsCard } from "./AISettingsCard";
import { DataSettingsCard } from "./DataSettingsCard";
import type { ScriptLang, SettingsTab } from "./types";
import { cn } from "@/lib/utils";

export type SettingsViewProps = {
  settingsTab: SettingsTab;
  onSettingsTabChange: (tab: SettingsTab) => void;
  feedURL: string;
  onFeedURLChange: (value: string) => void;
  newFeedFolderID: number | null;
  onNewFeedFolderIDChange: (value: number | null) => void;
  folders: Folder[];
  onCreateRootFolder: () => void;
  onAddFeed: () => void;
  onRefreshFeeds: () => void;
  onRefreshFeedsFromNetwork: () => void;
  onRefreshArticles: () => void;
  isRefreshingFeeds: boolean;
  isRefreshingArticles: boolean;
  scriptFeedID: number | null;
  feeds: Feed[];
  scriptLang: ScriptLang;
  scriptContent: string;
  onSelectScriptFeed: (feedID: number | null) => void;
  onUploadScriptFile: (event: ChangeEvent<HTMLInputElement>) => void;
  onScriptLangChange: (lang: ScriptLang) => void;
  onScriptContentChange: (value: string) => void;
  onSaveFeedScript: () => void;
  apiBase: string;
  onAPIBaseChange: (value: string) => void;
  onSaveAPIBase: () => void;
  networkProxyURL: string;
  onNetworkProxyURLChange: (value: string) => void;
  onSaveNetworkSettings: () => void;
  aiProtocol: "openai" | "anthropic";
  aiAPIKey: string;
  aiAPIKeyMasked: string;
  aiAPIKeyConfigured: boolean;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  onAIProtocolChange: (value: "openai" | "anthropic") => void;
  onAIAPIKeyChange: (value: string) => void;
  onAIBaseURLChange: (value: string) => void;
  onAIModelChange: (value: string) => void;
  onAITargetLangChange: (value: string) => void;
  onSaveAISettings: () => void;
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

const TAB_OPTIONS: Array<{ value: SettingsTab; label: string }> = [
  { value: "subscription", label: "订阅管理" },
  { value: "script", label: "脚本设置" },
  { value: "connection", label: "连接设置" },
  { value: "ai", label: "AI 设置" },
  { value: "data", label: "数据管理" },
];

export function SettingsView(props: SettingsViewProps) {
  const { settingsTab, onSettingsTabChange, ...rest } = props;

  return (
    <div className="flex h-full">
      {/* Sidebar nav */}
      <aside className="w-40 shrink-0 border-r bg-muted/30 p-3 flex flex-col gap-0.5">
        {TAB_OPTIONS.map((tab) => (
          <button
            key={tab.value}
            onClick={() => onSettingsTabChange(tab.value)}
            className={cn(
              "w-full text-left px-3 py-2 rounded-md text-sm transition-colors",
              settingsTab === tab.value
                ? "bg-background text-foreground font-medium shadow-sm"
                : "text-muted-foreground hover:bg-background/60 hover:text-foreground"
            )}
          >
            {tab.label}
          </button>
        ))}
      </aside>

      {/* Content */}
      <section className="flex-1 min-w-0 overflow-y-auto p-6">
        {/* Mobile tab picker */}
        <div className="sm:hidden mb-4">
          <label htmlFor="settingsMobileTab" className="block text-sm text-muted-foreground mb-1.5">
            设置分组
          </label>
          <select
            id="settingsMobileTab"
            value={settingsTab}
            onChange={(event) => onSettingsTabChange(event.target.value as SettingsTab)}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          >
            {TAB_OPTIONS.map((tab) => (
              <option key={tab.value} value={tab.value}>{tab.label}</option>
            ))}
          </select>
        </div>

        {settingsTab === "subscription" && (
          <SubscriptionSettingsCard
            feedURL={rest.feedURL}
            onFeedURLChange={rest.onFeedURLChange}
            newFeedFolderID={rest.newFeedFolderID}
            onNewFeedFolderIDChange={rest.onNewFeedFolderIDChange}
            folders={rest.folders}
            onCreateRootFolder={rest.onCreateRootFolder}
            onAddFeed={rest.onAddFeed}
            onRefreshFeeds={rest.onRefreshFeeds}
            onRefreshFeedsFromNetwork={rest.onRefreshFeedsFromNetwork}
            onRefreshArticles={rest.onRefreshArticles}
            isRefreshingFeeds={rest.isRefreshingFeeds}
            isRefreshingArticles={rest.isRefreshingArticles}
          />
        )}
        {settingsTab === "script" && (
          <ScriptSettingsCard
            scriptFeedID={rest.scriptFeedID}
            feeds={rest.feeds}
            scriptLang={rest.scriptLang}
            scriptContent={rest.scriptContent}
            onSelectScriptFeed={rest.onSelectScriptFeed}
            onUploadScriptFile={rest.onUploadScriptFile}
            onScriptLangChange={rest.onScriptLangChange}
            onScriptContentChange={rest.onScriptContentChange}
            onSaveFeedScript={rest.onSaveFeedScript}
          />
        )}
        {settingsTab === "connection" && (
          <ConnectionSettingsCard
            apiBase={rest.apiBase}
            onAPIBaseChange={rest.onAPIBaseChange}
            onSaveAPIBase={rest.onSaveAPIBase}
            networkProxyURL={rest.networkProxyURL}
            onNetworkProxyURLChange={rest.onNetworkProxyURLChange}
            onSaveNetworkSettings={rest.onSaveNetworkSettings}
          />
        )}
        {settingsTab === "ai" && (
          <AISettingsCard
            aiProtocol={rest.aiProtocol}
            aiAPIKey={rest.aiAPIKey}
            aiAPIKeyMasked={rest.aiAPIKeyMasked}
            aiAPIKeyConfigured={rest.aiAPIKeyConfigured}
            aiBaseURL={rest.aiBaseURL}
            aiModel={rest.aiModel}
            aiTargetLang={rest.aiTargetLang}
            onAIProtocolChange={rest.onAIProtocolChange}
            onAIAPIKeyChange={rest.onAIAPIKeyChange}
            onAIBaseURLChange={rest.onAIBaseURLChange}
            onAIModelChange={rest.onAIModelChange}
            onAITargetLangChange={rest.onAITargetLangChange}
            onSaveAISettings={rest.onSaveAISettings}
          />
        )}
        {settingsTab === "data" && (
          <DataSettingsCard
            articleRetentionDays={rest.articleRetentionDays}
            selectedArticleID={rest.selectedArticleID}
            selectedArticleTitle={rest.selectedArticleTitle}
            isRefreshingCurrentArticleAISummary={rest.isRefreshingCurrentArticleAISummary}
            onArticleRetentionDaysChange={rest.onArticleRetentionDaysChange}
            onSaveDataSettings={rest.onSaveDataSettings}
            onRefreshCurrentArticleAISummary={rest.onRefreshCurrentArticleAISummary}
            onClearCurrentArticleAISummary={rest.onClearCurrentArticleAISummary}
            onClearRecentAISummaries={rest.onClearRecentAISummaries}
            onRegenerateSummaries={rest.onRegenerateSummaries}
            onExportProfileJSON={rest.onExportProfileJSON}
            onExportOPML={rest.onExportOPML}
            onImportProfileJSON={rest.onImportProfileJSON}
            onImportOPML={rest.onImportOPML}
          />
        )}
      </section>
    </div>
  );
}
