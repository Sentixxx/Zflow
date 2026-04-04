import { useState } from "react";
import type { ScriptLang } from "@/components";

export function useSettingsState() {
  const [networkProxyURL, setNetworkProxyURL] = useState("");
  const [aiProtocol, setAIProtocol] = useState<"openai" | "anthropic">("openai");
  const [aiAPIKey, setAIAPIKey] = useState("");
  const [aiAPIKeyMasked, setAIAPIKeyMasked] = useState("");
  const [aiAPIKeyConfigured, setAIAPIKeyConfigured] = useState(false);
  const [aiBaseURL, setAIBaseURL] = useState("");
  const [aiModel, setAIModel] = useState("");
  const [aiTargetLang, setAITargetLang] = useState("zh-CN");
  const [articleRetentionDays, setArticleRetentionDays] = useState("90");
  const [scriptFeedID, setScriptFeedID] = useState<number | null>(null);
  const [scriptContent, setScriptContent] = useState("");
  const [scriptLang, setScriptLang] = useState<ScriptLang>("shell");
  const [scriptDirty, setScriptDirty] = useState(false);

  return {
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
  };
}
