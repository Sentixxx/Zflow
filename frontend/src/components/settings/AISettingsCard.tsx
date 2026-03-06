type AISettingsCardProps = {
  aiAPIKey: string;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  onAIAPIKeyChange: (value: string) => void;
  onAIBaseURLChange: (value: string) => void;
  onAIModelChange: (value: string) => void;
  onAITargetLangChange: (value: string) => void;
  onSaveAISettings: () => void;
};

export function AISettingsCard({
  aiAPIKey,
  aiBaseURL,
  aiModel,
  aiTargetLang,
  onAIAPIKeyChange,
  onAIBaseURLChange,
  onAIModelChange,
  onAITargetLangChange,
  onSaveAISettings,
}: AISettingsCardProps) {
  return (
    <div className="settings-page-inner settings-section-card">
      <h4 className="section-title">AI 设置</h4>
      <label htmlFor="aiApiKey">API Key</label>
      <input id="aiApiKey" type="password" value={aiAPIKey} placeholder="sk-..." onChange={(e) => onAIAPIKeyChange(e.target.value)} />
      <label htmlFor="aiBaseURL">Base URL（OpenAI 兼容）</label>
      <input id="aiBaseURL" value={aiBaseURL} placeholder="https://api.openai.com/v1" onChange={(e) => onAIBaseURLChange(e.target.value)} />
      <label htmlFor="aiModel">模型</label>
      <input id="aiModel" value={aiModel} placeholder="gpt-4o-mini" onChange={(e) => onAIModelChange(e.target.value)} />
      <label htmlFor="aiTargetLang">默认目标语言</label>
      <input id="aiTargetLang" value={aiTargetLang} placeholder="zh-CN" onChange={(e) => onAITargetLangChange(e.target.value)} />
      <div className="row">
        <button className="secondary" onClick={onSaveAISettings}>
          保存 AI 设置
        </button>
      </div>
    </div>
  );
}
