import { useEffect, useMemo, useState } from "react";
import { useLocation } from "wouter";
import { ApiClient } from "@/api";
import type { Feed } from "@/types";
import { SettingsView } from "@/components";
import type { ScriptLang, SettingsTab } from "@/components";
import type { RefreshFailure } from "@/components";
import { useReaderQueries } from "@/hooks/useReaderQueries";
import { useFeeds } from "@/hooks/useFeeds";
import { useEntries } from "@/hooks/useEntries";
import { useSettingsActions } from "@/hooks/useSettingsActions";
import { refreshFeedsBatch } from "@/services/feed-refresh-service";

const DEFAULT_API_BASE = "http://localhost:8080";

export function SettingsPage() {
  const [, setLocation] = useLocation();
  const [apiBase, setApiBase] = useState<string>(localStorage.getItem("zflow_api_base") || DEFAULT_API_BASE);
  const [networkProxyURL, setNetworkProxyURL] = useState<string>("");
  const [aiAPIKey, setAIAPIKey] = useState<string>("");
  const [aiBaseURL, setAIBaseURL] = useState<string>("");
  const [aiModel, setAIModel] = useState<string>("");
  const [aiTargetLang, setAITargetLang] = useState<string>("zh-CN");
  const [articleRetentionDays, setArticleRetentionDays] = useState<string>("90");
  const [feedURL, setFeedURL] = useState("");
  const [newFeedFolderID, setNewFeedFolderID] = useState<number | null>(null);
  const [scriptFeedID, setScriptFeedID] = useState<number | null>(null);
  const [scriptContent, setScriptContent] = useState<string>("");
  const [scriptLang, setScriptLang] = useState<ScriptLang>("shell");
  const [scriptDirty, setScriptDirty] = useState<boolean>(false);
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("subscription");
  const [isRefreshingArticles, setIsRefreshingArticles] = useState<boolean>(false);
  const [isRefreshingFeeds, setIsRefreshingFeeds] = useState<boolean>(false);
  const [refreshFailures, setRefreshFailures] = useState<RefreshFailure[]>([]);
  const [status, setStatus] = useState("准备就绪");
  const [error, setError] = useState("");

  const client = useMemo(() => new ApiClient(apiBase), [apiBase]);
  const { feedsQuery, foldersQuery, articlesInfiniteQuery } = useReaderQueries(apiBase);

  const setMessage = (message: string, isError = false) => {
    if (isError) {
      setError(message);
      setStatus("");
      return;
    }
    setError("");
    setStatus(message);
  };

  const { feeds, folders, loadFeeds, loadFolders } = useFeeds(client, feedsQuery, foldersQuery, setMessage);
  const { loadArticles } = useEntries(client, articlesInfiniteQuery, setMessage);

  const {
    handleSaveAPIBase,
    loadNetworkSettings,
    saveNetworkSettings,
    loadAISettings,
    saveAISettings,
    loadDataSettings,
    saveDataSettings,
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
    apiBase,
    networkProxyURL,
    aiAPIKey,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    setNetworkProxyURL,
    setAIAPIKey,
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
    setMessage,
  });

  const refreshFeedsFromNetwork = async () => {
    if (isRefreshingFeeds) {
      return;
    }
    setIsRefreshingFeeds(true);
    setRefreshFailures([]);
    try {
      setMessage("正在远端抓取订阅源...");
      const currentFeeds = await client.listFeeds();
      if (currentFeeds.length === 0) {
        setMessage("暂无订阅源可刷新");
        return;
      }
      const { successCount, failedCount, failures } = await refreshFeedsBatch(currentFeeds, (feedID) => client.refreshFeed(feedID));
      setRefreshFailures(failures);
      await Promise.all([loadFeeds({ silentStatus: true }), loadArticles({ silentStatus: true })]);
      if (failedCount > 0) {
        setMessage(`订阅源刷新完成：成功 ${successCount}，失败 ${failedCount}`);
      } else {
        setRefreshFailures([]);
        setMessage(`订阅源刷新完成：成功 ${successCount}`);
      }
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsRefreshingFeeds(false);
    }
  };

  const handleRefreshArticles = async () => {
    if (isRefreshingArticles) {
      return;
    }
    setIsRefreshingArticles(true);
    try {
      await loadArticles();
    } finally {
      setIsRefreshingArticles(false);
    }
  };

  const handleRefreshFeeds = async () => {
    await loadFeeds();
  };

  const addFeed = async () => {
    const url = feedURL.trim();
    if (!url) {
      setMessage("请输入 RSS/Atom URL", true);
      return;
    }
    try {
      setMessage("正在添加订阅并抓取...");
      await client.createFeed(url, newFeedFolderID);
      setFeedURL("");
      await Promise.all([loadFeeds(), loadFolders(), loadArticles()]);
      setMessage("订阅添加成功");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const createRootFolder = async () => {
    const name = window.prompt("分类名称", "新分类");
    if (!name || !name.trim()) {
      return;
    }
    try {
      await client.createFolder(name.trim());
      await Promise.all([loadFolders(), loadFeeds()]);
      setMessage("分类已创建");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

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
            aiAPIKey={aiAPIKey}
            aiBaseURL={aiBaseURL}
            aiModel={aiModel}
            aiTargetLang={aiTargetLang}
            onAIAPIKeyChange={setAIAPIKey}
            onAIBaseURLChange={setAIBaseURL}
            onAIModelChange={setAIModel}
            onAITargetLangChange={setAITargetLang}
            onSaveAISettings={saveAISettings}
            articleRetentionDays={articleRetentionDays}
            onArticleRetentionDaysChange={setArticleRetentionDays}
            onSaveDataSettings={saveDataSettings}
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
