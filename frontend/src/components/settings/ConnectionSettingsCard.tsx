import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type ConnectionSettingsCardProps = {
  apiBase: string;
  networkProxyURL: string;
  onAPIBaseChange: (value: string) => void;
  onNetworkProxyURLChange: (value: string) => void;
  onSaveAPIBase: () => void;
  onSaveNetworkSettings: () => void;
};

export function ConnectionSettingsCard({
  apiBase,
  networkProxyURL,
  onAPIBaseChange,
  onNetworkProxyURLChange,
  onSaveAPIBase,
  onSaveNetworkSettings,
}: ConnectionSettingsCardProps) {
  return (
    <div className="space-y-6">
      <div>
        <h4 className="text-base font-semibold mb-4">连接设置</h4>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="apiBase">API Base URL</Label>
            <div className="flex gap-2">
              <Input
                id="apiBase"
                value={apiBase}
                onChange={(e) => onAPIBaseChange(e.target.value)}
                className="flex-1"
              />
              <Button variant="outline" onClick={onSaveAPIBase}>保存</Button>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="networkProxy">网络代理 URL（可选，支持 http/https/socks5）</Label>
            <div className="flex gap-2">
              <Input
                id="networkProxy"
                value={networkProxyURL}
                placeholder="http://127.0.0.1:7890"
                onChange={(e) => onNetworkProxyURLChange(e.target.value)}
                className="flex-1"
              />
              <Button variant="outline" onClick={onSaveNetworkSettings}>保存代理</Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
