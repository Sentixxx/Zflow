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
    <div className="settings-page-inner settings-section-card">
      <h4 className="section-title">连接设置</h4>
      <label htmlFor="apiBase">API Base URL</label>
      <div className="row">
        <input id="apiBase" value={apiBase} onChange={(e) => onAPIBaseChange(e.target.value)} />
        <button className="secondary" onClick={onSaveAPIBase}>
          保存
        </button>
      </div>
      <label htmlFor="networkProxy">网络代理 URL（可选，支持 http/https/socks5）</label>
      <div className="row">
        <input id="networkProxy" value={networkProxyURL} placeholder="http://127.0.0.1:7890" onChange={(e) => onNetworkProxyURLChange(e.target.value)} />
        <button className="secondary" onClick={onSaveNetworkSettings}>
          保存代理
        </button>
      </div>
    </div>
  );
}
