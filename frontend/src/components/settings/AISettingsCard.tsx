import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type AISettingsCardProps = {
  aiProtocol: "openai" | "anthropic";
  aiAPIKey: string;
  aiAPIKeyMasked: string;
  aiAPIKeyConfigured: boolean;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  embeddingAPIKey: string;
  embeddingAPIKeyMasked: string;
  embeddingAPIKeyConfigured: boolean;
  embeddingBaseURL: string;
  embeddingModel: string;
  onAIProtocolChange: (value: "openai" | "anthropic") => void;
  onAIAPIKeyChange: (value: string) => void;
  onAIBaseURLChange: (value: string) => void;
  onAIModelChange: (value: string) => void;
  onAITargetLangChange: (value: string) => void;
  onEmbeddingAPIKeyChange: (value: string) => void;
  onEmbeddingBaseURLChange: (value: string) => void;
  onEmbeddingModelChange: (value: string) => void;
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
  embeddingAPIKey,
  embeddingAPIKeyMasked,
  embeddingAPIKeyConfigured,
  embeddingBaseURL,
  embeddingModel,
  onAIProtocolChange,
  onAIAPIKeyChange,
  onAIBaseURLChange,
  onAIModelChange,
  onAITargetLangChange,
  onEmbeddingAPIKeyChange,
  onEmbeddingBaseURLChange,
  onEmbeddingModelChange,
  onSaveAISettings,
}: AISettingsCardProps) {
  return (
    <div className="space-y-6">
      <div>
        <h4 className="text-base font-semibold mb-4">AI 设置</h4>
        <div className="space-y-4">
          <div className="rounded-lg border border-border/60 bg-muted/20 p-4 space-y-4">
            <div>
              <h5 className="text-sm font-medium">Chat / Translation</h5>
              <p className="text-xs text-muted-foreground mt-1">用于摘要、翻译、brief 和其他文本生成。</p>
            </div>
          <div className="space-y-1.5">
            <Label htmlFor="aiProtocol">协议</Label>
            <select
              id="aiProtocol"
              value={aiProtocol}
              onChange={(e) => onAIProtocolChange(e.target.value === "anthropic" ? "anthropic" : "openai")}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="openai">OpenAI 兼容</option>
              <option value="anthropic">Anthropic SDK</option>
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="aiApiKey">API Key</Label>
            <Input
              id="aiApiKey"
              type="password"
              value={aiAPIKey}
              placeholder={aiAPIKeyConfigured && aiAPIKeyMasked ? aiAPIKeyMasked : "sk-..."}
              onChange={(e) => onAIAPIKeyChange(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              {aiAPIKeyConfigured
                ? "GET 响应不会返回明文 API key；留空表示保持当前已保存的 key。"
                : "API key 只会在保存时上传，页面刷新后不会回显明文。"}
            </p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="aiBaseURL">Base URL</Label>
            <Input
              id="aiBaseURL"
              value={aiBaseURL}
              placeholder={aiProtocol === "anthropic" ? "https://api.minimaxi.com/anthropic" : "https://api.minimaxi.com/v1"}
              onChange={(e) => onAIBaseURLChange(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="aiModel">模型</Label>
            <Input
              id="aiModel"
              value={aiModel}
              placeholder={aiProtocol === "anthropic" ? "MiniMax-Text-01" : "MiniMax-M2.7"}
              onChange={(e) => onAIModelChange(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="aiTargetLang">默认目标语言</Label>
            <Input
              id="aiTargetLang"
              value={aiTargetLang}
              placeholder="zh-CN"
              onChange={(e) => onAITargetLangChange(e.target.value)}
            />
          </div>

          <p className="text-xs text-muted-foreground rounded-md bg-muted/50 px-3 py-2 border border-border/50">
            {aiProtocol === "anthropic"
              ? "Anthropic 模式会调用 /v1/messages，可填写兼容 Anthropic SDK 的服务，例如 Base URL https://api.minimaxi.com/anthropic。"
              : "OpenAI 模式会调用 /chat/completions，可直接填写 MiniMax OpenAI 兼容配置，例如 Base URL https://api.minimaxi.com/v1、模型 MiniMax-M2.7。"}
          </p>
          </div>

          <div className="rounded-lg border border-border/60 bg-muted/20 p-4 space-y-4">
            <div>
              <h5 className="text-sm font-medium">Embedding</h5>
              <p className="text-xs text-muted-foreground mt-1">用于向量检索、兴趣画像和 topic cluster。留空时后端会暂时回退到 Chat 配置。</p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="embeddingApiKey">Embedding API Key</Label>
              <Input
                id="embeddingApiKey"
                type="password"
                value={embeddingAPIKey}
                placeholder={embeddingAPIKeyConfigured && embeddingAPIKeyMasked ? embeddingAPIKeyMasked : "sk-..."}
                onChange={(e) => onEmbeddingAPIKeyChange(e.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="embeddingBaseURL">Embedding Base URL</Label>
              <Input
                id="embeddingBaseURL"
                value={embeddingBaseURL}
                placeholder="https://api.openai.com/v1"
                onChange={(e) => onEmbeddingBaseURLChange(e.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="embeddingModel">Embedding 模型</Label>
              <Input
                id="embeddingModel"
                value={embeddingModel}
                placeholder="text-embedding-3-small"
                onChange={(e) => onEmbeddingModelChange(e.target.value)}
              />
            </div>
          </div>

          <div>
            <Button variant="outline" onClick={onSaveAISettings}>保存 AI 设置</Button>
          </div>
        </div>
      </div>
    </div>
  );
}
