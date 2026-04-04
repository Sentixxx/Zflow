type AISettingsCardProps = {
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
};

export function AISettingsCard({
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
}: AISettingsCardProps) {
  return (
    <div className="settings-page-inner settings-section-card">
      <h4 className="section-title">AI 设置</h4>
      <label htmlFor="aiProtocol">协议</label>
      <select id="aiProtocol" value={aiProtocol} onChange={(e) => onAIProtocolChange(e.target.value === "anthropic" ? "anthropic" : "openai")}>
        <option value="openai">OpenAI 兼容</option>
        <option value="anthropic">Anthropic SDK</option>
      </select>
      <label htmlFor="aiApiKey">API Key</label>
      <input
        id="aiApiKey"
        type="password"
        value={aiAPIKey}
        placeholder={aiAPIKeyConfigured && aiAPIKeyMasked ? aiAPIKeyMasked : "sk-..."}
        onChange={(e) => onAIAPIKeyChange(e.target.value)}
      />
      <p className="settings-help-text">
        {aiAPIKeyConfigured ? "GET 响应不会返回明文 API key；留空表示保持当前已保存的 key。" : "API key 只会在保存时上传，页面刷新后不会回显明文。"}
      </p>
      <label htmlFor="aiBaseURL">Base URL</label>
      <input
        id="aiBaseURL"
        value={aiBaseURL}
        placeholder={aiProtocol === "anthropic" ? "https://api.minimaxi.com/anthropic" : "https://api.minimaxi.com/v1"}
        onChange={(e) => onAIBaseURLChange(e.target.value)}
      />
      <label htmlFor="aiModel">模型</label>
      <input id="aiModel" value={aiModel} placeholder={aiProtocol === "anthropic" ? "MiniMax-Text-01" : "MiniMax-M2.7"} onChange={(e) => onAIModelChange(e.target.value)} />
      <label htmlFor="aiTargetLang">默认目标语言</label>
      <input id="aiTargetLang" value={aiTargetLang} placeholder="zh-CN" onChange={(e) => onAITargetLangChange(e.target.value)} />
      <p className="settings-help-text">
        {aiProtocol === "anthropic"
          ? "Anthropic 模式会调用 `/v1/messages`，可填写兼容 Anthropic SDK 的服务，例如 Base URL `https://api.minimaxi.com/anthropic`。"
          : "OpenAI 模式会调用 `/chat/completions`，可直接填写 MiniMax OpenAI 兼容配置，例如 Base URL `https://api.minimaxi.com/v1`、模型 `MiniMax-M2.7`。"}
      </p>
      <div className="row">
        <button className="secondary" onClick={onSaveAISettings}>
          保存 AI 设置
        </button>
      </div>
    </div>
  );
}
