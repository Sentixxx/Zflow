import type { ChangeEvent } from "react";
import type { Feed, Folder } from "@/types";
import { SubscriptionSettingsCard } from "./SubscriptionSettingsCard";
import { ScriptSettingsCard } from "./ScriptSettingsCard";
import { ConnectionSettingsCard } from "./ConnectionSettingsCard";
import { AISettingsCard } from "./AISettingsCard";
import { DataSettingsCard } from "./DataSettingsCard";
import type { ScriptLang, SettingsTab } from "./types";

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

export function SettingsView(props: SettingsViewProps) {
  const {
    settingsTab,
    onSettingsTabChange,
    feedURL,
    onFeedURLChange,
    newFeedFolderID,
    onNewFeedFolderIDChange,
    folders,
    onCreateRootFolder,
    onAddFeed,
    onRefreshFeeds,
    onRefreshFeedsFromNetwork,
    onRefreshArticles,
    isRefreshingFeeds,
    isRefreshingArticles,
    scriptFeedID,
    feeds,
    scriptLang,
    scriptContent,
    onSelectScriptFeed,
    onUploadScriptFile,
    onScriptLangChange,
    onScriptContentChange,
    onSaveFeedScript,
    apiBase,
    onAPIBaseChange,
    onSaveAPIBase,
    networkProxyURL,
    onNetworkProxyURLChange,
    onSaveNetworkSettings,
    aiProtocol,
    aiAPIKey,
    aiAPIKeyMasked,
    aiAPIKeyConfigured,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    onAIProtocolChange,
    onAIAPIKeyChange,
    onAIBaseURLChange,
    onAIModelChange,
    onAITargetLangChange,
    onSaveAISettings,
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
  } = props;
  const tabOptions: Array<{ value: SettingsTab; label: string }> = [
    { value: "subscription", label: "订阅管理" },
    { value: "script", label: "脚本设置" },
    { value: "connection", label: "连接设置" },
    { value: "ai", label: "AI 设置" },
    { value: "data", label: "数据管理" },
  ];

  return (
    <div className="settings-modal-body">
      <aside className="settings-nav">
        {tabOptions.map((tab) => (
          <button key={tab.value} className={`settings-tab ${settingsTab === tab.value ? "active" : ""}`} onClick={() => onSettingsTabChange(tab.value)}>
            {tab.label}
          </button>
        ))}
      </aside>
      <section className="settings-page">
        <div className="settings-mobile-tab-picker">
          <label htmlFor="settingsMobileTab">设置分组</label>
          <select id="settingsMobileTab" value={settingsTab} onChange={(event) => onSettingsTabChange(event.target.value as SettingsTab)}>
            {tabOptions.map((tab) => (
              <option key={tab.value} value={tab.value}>
                {tab.label}
              </option>
            ))}
          </select>
        </div>
        {settingsTab === "subscription" && (
          <SubscriptionSettingsCard
            feedURL={feedURL}
            onFeedURLChange={onFeedURLChange}
            newFeedFolderID={newFeedFolderID}
            onNewFeedFolderIDChange={onNewFeedFolderIDChange}
            folders={folders}
            onCreateRootFolder={onCreateRootFolder}
            onAddFeed={onAddFeed}
            onRefreshFeeds={onRefreshFeeds}
            onRefreshFeedsFromNetwork={onRefreshFeedsFromNetwork}
            onRefreshArticles={onRefreshArticles}
            isRefreshingFeeds={isRefreshingFeeds}
            isRefreshingArticles={isRefreshingArticles}
          />
        )}
        {settingsTab === "script" && (
          <ScriptSettingsCard
            scriptFeedID={scriptFeedID}
            feeds={feeds}
            scriptLang={scriptLang}
            scriptContent={scriptContent}
            onSelectScriptFeed={onSelectScriptFeed}
            onUploadScriptFile={onUploadScriptFile}
            onScriptLangChange={onScriptLangChange}
            onScriptContentChange={onScriptContentChange}
            onSaveFeedScript={onSaveFeedScript}
          />
        )}
        {settingsTab === "connection" && (
          <ConnectionSettingsCard
            apiBase={apiBase}
            onAPIBaseChange={onAPIBaseChange}
            onSaveAPIBase={onSaveAPIBase}
            networkProxyURL={networkProxyURL}
            onNetworkProxyURLChange={onNetworkProxyURLChange}
            onSaveNetworkSettings={onSaveNetworkSettings}
          />
        )}
        {settingsTab === "ai" && (
          <AISettingsCard
            aiProtocol={aiProtocol}
            aiAPIKey={aiAPIKey}
            aiAPIKeyMasked={aiAPIKeyMasked}
            aiAPIKeyConfigured={aiAPIKeyConfigured}
            aiBaseURL={aiBaseURL}
            aiModel={aiModel}
            aiTargetLang={aiTargetLang}
            onAIProtocolChange={onAIProtocolChange}
            onAIAPIKeyChange={onAIAPIKeyChange}
            onAIBaseURLChange={onAIBaseURLChange}
            onAIModelChange={onAIModelChange}
            onAITargetLangChange={onAITargetLangChange}
            onSaveAISettings={onSaveAISettings}
          />
        )}
        {settingsTab === "data" && (
          <DataSettingsCard
            articleRetentionDays={articleRetentionDays}
            selectedArticleID={selectedArticleID}
            selectedArticleTitle={selectedArticleTitle}
            isRefreshingCurrentArticleAISummary={isRefreshingCurrentArticleAISummary}
            onArticleRetentionDaysChange={onArticleRetentionDaysChange}
            onSaveDataSettings={onSaveDataSettings}
            onRefreshCurrentArticleAISummary={onRefreshCurrentArticleAISummary}
            onClearCurrentArticleAISummary={onClearCurrentArticleAISummary}
            onClearRecentAISummaries={onClearRecentAISummaries}
            onRegenerateSummaries={onRegenerateSummaries}
            onExportProfileJSON={onExportProfileJSON}
            onExportOPML={onExportOPML}
            onImportProfileJSON={onImportProfileJSON}
            onImportOPML={onImportOPML}
          />
        )}
      </section>
    </div>
  );
}
