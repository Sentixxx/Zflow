export type RefreshFailure = {
  feedID: number;
  feedTitle: string;
  reason: string;
};

type RefreshFailureBannerProps = {
  failures: RefreshFailure[];
  onClose: () => void;
};

export function RefreshFailureBanner({ failures, onClose }: RefreshFailureBannerProps) {
  if (failures.length === 0) {
    return null;
  }

  return (
    <section className="refresh-failure-banner" role="status" aria-live="polite">
      <div className="refresh-failure-head">
        <strong>以下订阅源远端抓取失败（{failures.length}）</strong>
        <button className="secondary mini-btn" onClick={onClose}>
          关闭
        </button>
      </div>
      <ul className="refresh-failure-list">
        {failures.slice(0, 6).map((item) => (
          <li key={`${item.feedID}-${item.feedTitle}`}>
            <span className="feed-name">{item.feedTitle}</span>
            <span className="reason">{item.reason}</span>
          </li>
        ))}
        {failures.length > 6 && <li className="more">还有 {failures.length - 6} 个失败源，请查看后端日志</li>}
      </ul>
    </section>
  );
}
