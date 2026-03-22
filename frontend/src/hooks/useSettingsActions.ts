import { useState } from "react";
import type { ChangeEvent, Dispatch, SetStateAction } from "react";
import type { ApiClient } from "@/api";
import type { Article, Feed } from "@/types";
import type { ScriptLang } from "@/components/settings";

function normalizeScriptLang(raw: string | undefined): ScriptLang {
  if (raw === "python" || raw === "javascript") {
    return raw;
  }
  return "shell";
}

function scriptLangByFileName(name: string): ScriptLang | null {
  const lower = name.toLowerCase();
  if (lower.endsWith(".py")) return "python";
  if (lower.endsWith(".js") || lower.endsWith(".mjs") || lower.endsWith(".cjs")) return "javascript";
  if (lower.endsWith(".sh") || lower.endsWith(".bash") || lower.endsWith(".zsh")) return "shell";
  return null;
}

type UseSettingsActionsParams = {
  client: ApiClient;
  feeds: Feed[];
  selectedArticleID: number | null;
  selectedArticleTitle?: string;
  selectedArticleSummaryStatus?: string;
  apiBase: string;
  networkProxyURL: string;
  aiProtocol: "openai" | "anthropic";
  aiAPIKey: string;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  articleRetentionDays: string;
  scriptFeedID: number | null;
  scriptContent: string;
  scriptLang: ScriptLang;
  setNetworkProxyURL: Dispatch<SetStateAction<string>>;
  setAIProtocol: Dispatch<SetStateAction<"openai" | "anthropic">>;
  setAIAPIKey: Dispatch<SetStateAction<string>>;
  setAIBaseURL: Dispatch<SetStateAction<string>>;
  setAIModel: Dispatch<SetStateAction<string>>;
  setAITargetLang: Dispatch<SetStateAction<string>>;
  setArticleRetentionDays: Dispatch<SetStateAction<string>>;
  setScriptFeedID: Dispatch<SetStateAction<number | null>>;
  setScriptContent: Dispatch<SetStateAction<string>>;
  setScriptLang: Dispatch<SetStateAction<ScriptLang>>;
  setScriptDirty: Dispatch<SetStateAction<boolean>>;
  loadFeeds: (opts?: { silentStatus?: boolean }) => Promise<unknown>;
  loadFolders: (opts?: { silentStatus?: boolean }) => Promise<unknown>;
  loadArticles: (opts?: { silentStatus?: boolean }) => Promise<unknown>;
  onSelectedArticleUpdated?: (article: Article | null) => void;
  setMessage: (message: string, isError?: boolean) => void;
};

export function useSettingsActions({
  client,
  feeds,
  selectedArticleID,
  selectedArticleTitle,
  selectedArticleSummaryStatus,
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
  onSelectedArticleUpdated,
  setMessage,
}: UseSettingsActionsParams) {
  const [isRefreshingCurrentArticleAISummary, setIsRefreshingCurrentArticleAISummary] = useState(false);

  const selectedArticleLabel = (selectedArticleTitle || "").trim() || (selectedArticleID != null ? `#${selectedArticleID}` : "当前文章");
  const summaryStatusLabel = (status: string | undefined) => {
    switch ((status || "").trim()) {
      case "ready":
        return "AI 摘要";
      case "fallback":
        return "快速摘要";
      default:
        return "摘要";
    }
  };

  const summarizeDebug = (article: Article) => {
    const debug = article.summary_debug;
    if (!debug) {
      return "";
    }
    const parts: string[] = [];
    if (debug.strategy) {
      parts.push(`策略 ${debug.strategy}`);
    }
    if (debug.query_mode) {
      parts.push(`query ${debug.query_mode}`);
    }
    if (typeof debug.chunk_count === "number" && debug.chunk_count > 0) {
      parts.push(`chunks ${debug.chunk_count}`);
    }
    if (typeof debug.window_count === "number" && debug.window_count > 0) {
      parts.push(`windows ${debug.window_count}`);
    }
    if (debug.rewrite_passed) {
      parts.push("已压缩为扫读版");
    }
    if (typeof debug.final_sentence_closed === "boolean") {
      parts.push(debug.final_sentence_closed ? "完整收尾" : "可能未完整收尾");
    }
    return parts.join(" · ");
  };

  const refreshSelectedArticle = async () => {
    if (selectedArticleID == null || !onSelectedArticleUpdated) {
      return;
    }
    const article = await client.getArticle(selectedArticleID);
    onSelectedArticleUpdated(article);
  };

  const handleSaveAPIBase = () => {
    localStorage.setItem("zflow_api_base", apiBase);
    setMessage("API Base 已保存");
  };

  const loadNetworkSettings = async () => {
    try {
      const data = await client.getNetworkSettings();
      setNetworkProxyURL((data.proxy_url || "").trim());
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const saveNetworkSettings = async () => {
    try {
      const data = await client.updateNetworkSettings(networkProxyURL.trim());
      setNetworkProxyURL((data.proxy_url || "").trim());
      setMessage("网络代理设置已保存并应用");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const loadAISettings = async () => {
    try {
      const data = await client.getAISettings();
      setAIProtocol(data.protocol === "anthropic" ? "anthropic" : "openai");
      setAIAPIKey((data.api_key || "").trim());
      setAIBaseURL((data.base_url || "").trim());
      setAIModel((data.model || "").trim());
      setAITargetLang((data.target_lang || "zh-CN").trim() || "zh-CN");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const saveAISettings = async () => {
    try {
      const data = await client.updateAISettings({
        protocol: aiProtocol,
        api_key: aiAPIKey.trim(),
        base_url: aiBaseURL.trim(),
        model: aiModel.trim(),
        target_lang: aiTargetLang.trim() || "zh-CN",
      });
      const savedProtocol = data.protocol === "anthropic" ? "anthropic" : "openai";
      if (savedProtocol !== aiProtocol) {
        setMessage("AI 协议保存失败，请重试", true);
        return;
      }
      setAIProtocol(savedProtocol);
      setAIAPIKey((data.api_key || "").trim());
      setAIBaseURL((data.base_url || "").trim());
      setAIModel((data.model || "").trim());
      setAITargetLang((data.target_lang || "zh-CN").trim() || "zh-CN");
      setMessage("AI 设置已保存");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const loadDataSettings = async () => {
    try {
      const data = await client.getDataSettings();
      const days = Number(data.retention_days ?? 90);
      setArticleRetentionDays(String(Number.isFinite(days) && days > 0 ? Math.floor(days) : 90));
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const saveDataSettings = async () => {
    const days = Number(articleRetentionDays);
    if (!Number.isInteger(days) || days <= 0 || days > 3650) {
      setMessage("文章保留天数需为 1-3650 的整数", true);
      return;
    }
    try {
      const data = await client.updateDataSettings(days);
      const value = Number(data.retention_days ?? days);
      setArticleRetentionDays(String(value));
      setMessage("数据保留策略已保存");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const regenerateSummaries = async () => {
    try {
      const data = await client.regenerateSummaries();
      setMessage(`已重生成最近 ${Number(data.refreshed ?? 0)} 篇文章摘要`);
      await loadArticles({ silentStatus: true });
      await refreshSelectedArticle();
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const clearCurrentArticleAISummary = async () => {
    if (selectedArticleID == null) {
      setMessage("当前没有选中的文章", true);
      return;
    }
    try {
      setMessage(`正在清除当前文章 AI 摘要：${selectedArticleLabel}`);
      const article = await client.clearArticleAISummary(selectedArticleID);
      onSelectedArticleUpdated?.(article);
      await loadArticles({ silentStatus: true });
      await refreshSelectedArticle();
      setMessage(`已清除当前文章 AI 摘要：${selectedArticleLabel} · ${summaryStatusLabel(article.display_summary_status)}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const refreshCurrentArticleAISummary = async () => {
    if (selectedArticleID == null) {
      setMessage("当前没有选中的文章", true);
      return;
    }
    if (isRefreshingCurrentArticleAISummary) {
      setMessage(`当前文章摘要仍在刷新中：${selectedArticleLabel}`);
      return;
    }
    setIsRefreshingCurrentArticleAISummary(true);
    try {
      setMessage(`正在刷新当前文章摘要：${selectedArticleLabel} · 先前状态 ${summaryStatusLabel(selectedArticleSummaryStatus)}`);
      const article = await client.refreshArticleAISummary(selectedArticleID);
      onSelectedArticleUpdated?.(article);
      await loadArticles({ silentStatus: true });
      await refreshSelectedArticle();
      const debugText = summarizeDebug(article);
      setMessage(
        `当前文章摘要已刷新：${selectedArticleLabel} · ${summaryStatusLabel(article.display_summary_status)}${debugText ? ` · ${debugText}` : ""}`,
      );
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsRefreshingCurrentArticleAISummary(false);
    }
  };

  const clearRecentAISummaries = async () => {
    try {
      setMessage("正在清除最近 100 篇文章的 AI 摘要...");
      const data = await client.clearRecentAISummaries();
      await loadArticles({ silentStatus: true });
      await refreshSelectedArticle();
      setMessage(`已清除最近 ${Number(data.cleared ?? 0)} 篇文章的 AI 摘要`);
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const selectScriptFeed = (feedID: number | null) => {
    setScriptFeedID(feedID);
    setScriptDirty(false);
    if (feedID == null) {
      setScriptContent("");
      setScriptLang("shell");
      return;
    }
    const feed = feeds.find((item) => item.id === feedID);
    setScriptContent(feed?.custom_script || "");
    setScriptLang(normalizeScriptLang(feed?.custom_script_lang));
  };

  const saveFeedScript = async () => {
    if (scriptFeedID == null) {
      setMessage("请先选择订阅源", true);
      return;
    }
    try {
      await client.updateFeedScript(scriptFeedID, scriptContent, scriptLang);
      setScriptDirty(false);
      await loadFeeds();
      setMessage("脚本已保存");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const uploadScriptFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    const text = await file.text();
    const detectedLang = scriptLangByFileName(file.name);
    setScriptContent(text);
    if (detectedLang) {
      setScriptLang(detectedLang);
    }
    setScriptDirty(true);
    event.target.value = "";
  };

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(url);
  };

  const exportProfileJSON = async () => {
    try {
      const blob = await client.exportProfile();
      downloadBlob(blob, "zflow-profile.json");
      setMessage("已导出个人配置");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const exportOPML = async () => {
    try {
      const blob = await client.exportOPML();
      downloadBlob(blob, "zflow-subscriptions.opml");
      setMessage("已导出 OPML");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const importProfileJSON = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    try {
      const raw = await file.text();
      const result = await client.importProfile(raw);
      await Promise.all([loadFolders(), loadFeeds(), loadArticles()]);
      setMessage(`个人配置导入完成：新增订阅 ${result.imported_feeds ?? 0}，更新订阅 ${result.updated_feeds ?? 0}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      event.target.value = "";
    }
  };

  const importOPML = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    try {
      const raw = await file.text();
      const result = await client.importOPML(raw);
      await Promise.all([loadFolders(), loadFeeds(), loadArticles()]);
      setMessage(`OPML 导入完成：新增订阅 ${result.imported_feeds ?? 0}，更新订阅 ${result.updated_feeds ?? 0}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      event.target.value = "";
    }
  };

  return {
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
  };
}
