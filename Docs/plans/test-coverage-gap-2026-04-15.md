# Test Coverage Gap — 2026-04-15

契约来源：`.phrase/phases/phase-rss-llm-reader-20260225/`（spec / tech-refer）及 `CLAUDE.md` 规则。

## 后端缺口

| 文件 | 不变量 / 契约 | 计划测试名 |
|------|-------------|-----------|
| `feedparser/parser.go` | Atom 格式解析：rel=alternate 优先，无 rel 兜底，多 link 场景 | `TestParseFeed_When_AtomFeed_Should_PickAlternateLinkFirst` |
| `feedparser/parser.go` | Atom 缺 published 时回退 updated | `TestParseFeed_When_AtomEntryMissingPublished_Should_FallbackToUpdated` |
| `feedparser/parser.go` | RSS 缺 title/link 时容错，不 panic | `TestParseFeed_When_RSSItemMissingTitleAndLink_Should_NotPanic` |
| `feedparser/parser.go` | ErrUnsupportedFeed：非 RSS/Atom 根元素 | `TestParseFeed_When_UnknownRoot_Should_ReturnErrUnsupportedFeed` |
| `feedparser/parser.go` | enclosure 图片封面 URL 提取 | `TestParseFeed_When_RSSItemHasEnclosure_Should_ExtractCoverURL` |
| `service/feed_refresh_service.go` | 304 Not Modified：ETag/If-Modified-Since 发出后收到 304 不覆盖已有 title/items | `TestFeedRefreshService_When_Server304_Should_NotModifyExistingArticles` |
| `service/feed_refresh_service.go` | 304 后 ETag 保留（服务端 ETag 为空时回退已有 ETag）| `TestFeedRefreshService_When_Server304WithNoNewETag_Should_PreserveOldETag` |
| `service/feed_refresh_service.go` | script ok=false 时回退到原始 item（现有测试已覆盖 script exit 1，此处测 ok=false JSON 返回）| `TestFeedRefreshService_When_ScriptReturnsOkFalse_Should_FallbackToRawItem` |
| `service/feed_refresh_service.go` | retention_days setting 缺失时使用默认值 7 | `TestFeedRefreshService_When_RetentionDaysSettingMissing_Should_UseDefault7` |
| `scheduler/article_score_refresh.go` | 默认 batchSize=50, interval=1min | `TestArticleScoreRefreshScheduler_When_ZeroParams_Should_UseDefaults` |
| `scheduler/article_score_refresh.go` | ctx 取消后调度停止，runner 不再调用 | `TestArticleScoreRefreshScheduler_When_CtxCancelled_Should_Stop` |

## 前端缺口

| 文件 | 不变量 / 契约 | 计划测试名 |
|------|-------------|-----------|
| `lib/sanitize.ts` | script/iframe 被 DOMPurify 剔除 | `sanitizeRichHTML > removes script and iframe tags` |
| `lib/sanitize.ts` | 安全 href 协议保留（https:// img src 等）| `sanitizeRichHTML > keeps safe href and img attributes` |
| `lib/sanitize.ts` | undefined/空字符串入参返回空字符串 | `sanitizeRichHTML > returns empty string for empty or undefined input` |
| `lib/feed-utils.ts` | feedHost 提取域名；无效 URL 返回空 | `feedHost > extracts host from valid URL / returns empty for invalid` |
| `lib/feed-utils.ts` | buildFeedIconURLByHost 按 host 去重，第一个 icon 优先 | `buildFeedIconURLByHost > deduplicates by host and prefixes apiBase` |
| `services/feed-refresh-service.ts` | refreshFeedsBatch 汇聚成功/失败，失败携带 feedTitle | `refreshFeedsBatch > aggregates success and failure results` |
| `services/feed-refresh-service.ts` | 全成功：failedCount=0，failures=[] | `refreshFeedsBatch > returns zero failures when all succeed` |
| `stores/useReaderStore.ts` | setSelectedFeedID 不清除 selectedFolderID（各自独立）| `useReaderStore > setSelectedFeedID updates only feedID` |
| `stores/useReaderStore.ts` | 默认值：sortMode=latest, readFilter=all | `useReaderStore > initializes with correct defaults` |
