import { useEffect, useState } from "react";
import { useLocation } from "wouter";
import { SettingsView } from "@/components";
import type { SettingsTab } from "@/components";
import { useReaderBootstrap } from "@/hooks/useReaderBootstrap";
import { useSettingsState } from "@/hooks/useSettingsState";
import { useSettingsActions } from "@/hooks/useSettingsActions";
import { resolveInitialAPIBase } from "@/lib/api-base";

export function SettingsPage() {
  const [, setLocation] = useLocation();
  const [apiBase, setApiBase] = useState<string>(() => resolveInitialAPIBase(localStorage.getItem("zflow_api_base"), window.location.hostname));
  const settingsState = useSettingsState();
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("subscription");
  const {
    networkProxyURL,
    aiProtocol,
    aiAPIKey,
    aiAPIKeyMasked,
    aiAPIKeyConfigured,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    scriptDirty,
    setNetworkProxyURL,
    setAIProtocol,
    setAIAPIKey,
    setAIAPIKeyMasked,
    setAIAPIKeyConfigured,
    setAIBaseURL,
    setAIModel,
    setAITargetLang,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
  } = settingsState;

  const {
    client,
    feeds,
    folders,
    loadFeeds,
    loadFolders,
    loadArticles,
    feedURL,
    setFeedURL,
    newFeedFolderID,
    setNewFeedFolderID,
    isRefreshingFeeds,
    isRefreshingArticles,
    refreshFailures,
    refreshFeedsFromNetwork,
    handleRefreshArticles,
    handleRefreshFeeds,
    addFeed,
    createRootFolder,
    setMessage,
    status,
    error,
  } = useReaderBootstrap(apiBase);

  const {
    handleSaveAPIBase,
    loadNetworkSettings,
    saveNetworkSettings,
    loadAISettings,
    saveAISettings,
    loadDataSettings,
    saveDataSettings,
    regenerateSummaries,
    refreshCurrentArticleAISummary,
    clearCurrentArticleAISummary,
    clearRecentAISummaries,
    isRefreshingCurrentArticleAISummary,
    selectScriptFeed,
    saveFeedScript,
    uploadScriptFile,
    exportProfileJSON,
    exportOPML,
    importProfileJSON,
    importOPML,
  } = useSettingsActions({
    client,
    feeds,
    selectedArticleID: null,
    selectedArticleTitle: "",
    selectedArticleSummaryStatus: "",
    apiBase,
    networkProxyURL,
    aiProtocol,
    aiAPIKey,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    setNetworkProxyURL,
    setAIProtocol,
    setAIAPIKey,
    setAIAPIKeyMasked,
    setAIAPIKeyConfigured,
    setAIBaseURL,
    setAIModel,
    setAITargetLang,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
    loadFeeds,
    loadFolders,
    loadArticles,
    onSelectedArticleUpdated: undefined,
    setMessage,
  });

  useEffect(() => {
    const bootstrap = async () => {
      await Promise.all([loadFolders(), loadFeeds(), loadArticles(), loadNetworkSettings(), loadAISettings(), loadDataSettings()]);
    };
    void bootstrap();
  }, []);

  useEffect(() => {
    if (feeds.length === 0) {
      setScriptFeedID(null);
      setScriptContent("");
      return;
    }
    if (scriptFeedID == null || !feeds.some((feed) => feed.id === scriptFeedID)) {
      const nextID = feeds[0].id;
      setScriptFeedID(nextID);
      const nextFeed = feeds.find((feed) => feed.id === nextID);
      setScriptContent(nextFeed?.custom_script || "");
      setScriptLang(nextFeed?.custom_script_lang === "python" || nextFeed?.custom_script_lang === "javascript" ? nextFeed.custom_script_lang : "shell");
      setScriptDirty(false);
      return;
    }
    if (scriptDirty) {
      return;
    }
    const current = feeds.find((feed) => feed.id === scriptFeedID);
    if (current) {
      setScriptContent(current.custom_script || "");
      setScriptLang(current.custom_script_lang === "python" || current.custom_script_lang === "javascript" ? current.custom_script_lang : "shell");
    }
  }, [feeds, scriptFeedID, scriptDirty]);

  return (
    <div className="shell">
      <main className="layout" style={{ gridTemplateColumns: "minmax(0, 1fr)" }}>
        <section className="panel detail-panel" style={{ minHeight: "calc(100vh - 88px)" }}>
          <div className="settings-modal-header" style={{ borderBottom: "1px solid var(--line)", marginBottom: 10 }}>
            <h3>设置中心</h3>
            <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
              <span className={error ? "status error" : "status"}>{error || status}</span>
              <button className="secondary" onClick={() => setLocation("/")}>
                返回阅读器
              </button>
            </div>
          </div>
          <SettingsView
            settingsTab={settingsTab}
            onSettingsTabChange={setSettingsTab}
            feedURL={feedURL}
            onFeedURLChange={setFeedURL}
            newFeedFolderID={newFeedFolderID}
            onNewFeedFolderIDChange={setNewFeedFolderID}
            folders={folders}
            onCreateRootFolder={createRootFolder}
            onAddFeed={addFeed}
            onRefreshFeeds={handleRefreshFeeds}
            onRefreshFeedsFromNetwork={refreshFeedsFromNetwork}
            onRefreshArticles={handleRefreshArticles}
            isRefreshingFeeds={isRefreshingFeeds}
            isRefreshingArticles={isRefreshingArticles}
            scriptFeedID={scriptFeedID}
            feeds={feeds}
            scriptLang={scriptLang}
            scriptContent={scriptContent}
            onSelectScriptFeed={selectScriptFeed}
            onUploadScriptFile={uploadScriptFile}
            onScriptLangChange={(lang) => {
              setScriptLang(lang);
              setScriptDirty(true);
            }}
            onScriptContentChange={(value) => {
              setScriptContent(value);
              setScriptDirty(true);
            }}
            onSaveFeedScript={saveFeedScript}
            apiBase={apiBase}
            onAPIBaseChange={setApiBase}
            onSaveAPIBase={handleSaveAPIBase}
            networkProxyURL={networkProxyURL}
            onNetworkProxyURLChange={setNetworkProxyURL}
            onSaveNetworkSettings={saveNetworkSettings}
            aiProtocol={aiProtocol}
            aiAPIKey={aiAPIKey}
            aiAPIKeyMasked={aiAPIKeyMasked}
            aiAPIKeyConfigured={aiAPIKeyConfigured}
            aiBaseURL={aiBaseURL}
            aiModel={aiModel}
            aiTargetLang={aiTargetLang}
            onAIProtocolChange={setAIProtocol}
            onAIAPIKeyChange={setAIAPIKey}
            onAIBaseURLChange={setAIBaseURL}
            onAIModelChange={setAIModel}
            onAITargetLangChange={setAITargetLang}
            onSaveAISettings={saveAISettings}
            articleRetentionDays={articleRetentionDays}
            selectedArticleID={null}
            selectedArticleTitle=""
            isRefreshingCurrentArticleAISummary={isRefreshingCurrentArticleAISummary}
            onArticleRetentionDaysChange={setArticleRetentionDays}
            onSaveDataSettings={saveDataSettings}
            onRefreshCurrentArticleAISummary={refreshCurrentArticleAISummary}
            onClearCurrentArticleAISummary={clearCurrentArticleAISummary}
            onClearRecentAISummaries={clearRecentAISummaries}
            onRegenerateSummaries={regenerateSummaries}
            onExportProfileJSON={exportProfileJSON}
            onExportOPML={exportOPML}
            onImportProfileJSON={importProfileJSON}
            onImportOPML={importOPML}
          />
          {refreshFailures.length > 0 && (
            <div style={{ marginTop: 12 }}>
              {refreshFailures.map((failure) => (
                <div key={`${failure.feedID}-${failure.feedTitle}`} className="error-line">
                  {failure.feedTitle}: {failure.reason}
                </div>
              ))}
            </div>
          )}
        </section>
      </main>
    </div>
  );
}
