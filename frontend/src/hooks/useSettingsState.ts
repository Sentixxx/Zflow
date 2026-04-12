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
  const [embeddingAPIKey, setEmbeddingAPIKey] = useState("");
  const [embeddingAPIKeyMasked, setEmbeddingAPIKeyMasked] = useState("");
  const [embeddingAPIKeyConfigured, setEmbeddingAPIKeyConfigured] = useState(false);
  const [embeddingBaseURL, setEmbeddingBaseURL] = useState("");
  const [embeddingModel, setEmbeddingModel] = useState("");
  const [articleRetentionDays, setArticleRetentionDays] = useState("7");
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
    embeddingAPIKey,
    embeddingAPIKeyMasked,
    embeddingAPIKeyConfigured,
    embeddingBaseURL,
    embeddingModel,
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
    setEmbeddingAPIKey,
    setEmbeddingAPIKeyMasked,
    setEmbeddingAPIKeyConfigured,
    setEmbeddingBaseURL,
    setEmbeddingModel,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
  };
}
