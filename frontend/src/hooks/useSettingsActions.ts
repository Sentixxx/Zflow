import type { ChangeEvent, Dispatch, SetStateAction } from "react";
import type { ApiClient } from "@/api";
import type { Feed } from "@/types";
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
  apiBase: string;
  networkProxyURL: string;
  aiAPIKey: string;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  articleRetentionDays: string;
  scriptFeedID: number | null;
  scriptContent: string;
  scriptLang: ScriptLang;
  setNetworkProxyURL: Dispatch<SetStateAction<string>>;
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
  setMessage: (message: string, isError?: boolean) => void;
};

export function useSettingsActions({
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
}: UseSettingsActionsParams) {
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
        api_key: aiAPIKey.trim(),
        base_url: aiBaseURL.trim(),
        model: aiModel.trim(),
        target_lang: aiTargetLang.trim() || "zh-CN",
      });
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
    selectScriptFeed,
    saveFeedScript,
    uploadScriptFile,
    exportProfileJSON,
    exportOPML,
    importProfileJSON,
    importOPML,
  };
}
