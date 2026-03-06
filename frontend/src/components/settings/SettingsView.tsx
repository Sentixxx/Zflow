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
  aiAPIKey: string;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  onAIAPIKeyChange: (value: string) => void;
  onAIBaseURLChange: (value: string) => void;
  onAIModelChange: (value: string) => void;
  onAITargetLangChange: (value: string) => void;
  onSaveAISettings: () => void;
  articleRetentionDays: string;
  onArticleRetentionDaysChange: (value: string) => void;
  onSaveDataSettings: () => void;
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
    aiAPIKey,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    onAIAPIKeyChange,
    onAIBaseURLChange,
    onAIModelChange,
    onAITargetLangChange,
    onSaveAISettings,
    articleRetentionDays,
    onArticleRetentionDaysChange,
    onSaveDataSettings,
    onExportProfileJSON,
    onExportOPML,
    onImportProfileJSON,
    onImportOPML,
  } = props;

  return (
    <div className="settings-modal-body">
      <aside className="settings-nav">
        <button className={`settings-tab ${settingsTab === "subscription" ? "active" : ""}`} onClick={() => onSettingsTabChange("subscription")}>订阅管理</button>
        <button className={`settings-tab ${settingsTab === "script" ? "active" : ""}`} onClick={() => onSettingsTabChange("script")}>脚本设置</button>
        <button className={`settings-tab ${settingsTab === "connection" ? "active" : ""}`} onClick={() => onSettingsTabChange("connection")}>连接设置</button>
        <button className={`settings-tab ${settingsTab === "ai" ? "active" : ""}`} onClick={() => onSettingsTabChange("ai")}>AI 设置</button>
        <button className={`settings-tab ${settingsTab === "data" ? "active" : ""}`} onClick={() => onSettingsTabChange("data")}>数据管理</button>
      </aside>
      <section className="settings-page">
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
            aiAPIKey={aiAPIKey}
            aiBaseURL={aiBaseURL}
            aiModel={aiModel}
            aiTargetLang={aiTargetLang}
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
            onArticleRetentionDaysChange={onArticleRetentionDaysChange}
            onSaveDataSettings={onSaveDataSettings}
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
