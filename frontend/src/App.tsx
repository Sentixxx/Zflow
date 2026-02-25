import { useMemo, useState } from "react";
import { ApiClient } from "./api";
import type { Article, Feed } from "./types";

const DEFAULT_API_BASE = "http://localhost:8080";

export function App() {
  const [apiBase, setApiBase] = useState<string>(localStorage.getItem("zflow_api_base") || DEFAULT_API_BASE);
  const [feedURL, setFeedURL] = useState("");
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [articles, setArticles] = useState<Article[]>([]);
  const [selectedArticle, setSelectedArticle] = useState<Article | null>(null);
  const [status, setStatus] = useState("准备就绪");
  const [error, setError] = useState("");

  const client = useMemo(() => new ApiClient(apiBase), [apiBase]);

  const setMessage = (message: string, isError = false) => {
    if (isError) {
      setError(message);
      setStatus("");
      return;
    }
    setError("");
    setStatus(message);
  };

  const handleSaveAPIBase = () => {
    localStorage.setItem("zflow_api_base", apiBase);
    setMessage("API Base 已保存");
  };

  const loadFeeds = async () => {
    try {
      const data = await client.listFeeds();
      setFeeds(data);
      setMessage("订阅列表已刷新");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const loadArticles = async () => {
    try {
      const data = await client.listArticles();
      setArticles(data);
      setMessage("文章列表已刷新");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const addFeed = async () => {
    const url = feedURL.trim();
    if (!url) {
      setMessage("请输入 RSS/Atom URL", true);
      return;
    }
    try {
      setMessage("正在添加订阅并抓取...");
      await client.createFeed(url);
      setFeedURL("");
      await Promise.all([loadFeeds(), loadArticles()]);
      setMessage("订阅添加成功");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const selectArticle = async (id: number) => {
    try {
      const article = await client.getArticle(id);
      setSelectedArticle(article);
      setMessage(`已打开文章 #${id}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const markRead = async (read: boolean) => {
    if (!selectedArticle) {
      return;
    }
    try {
      const updated = await client.setArticleRead(selectedArticle.id, read);
      setSelectedArticle(updated);
      await loadArticles();
      setMessage(`文章已标记为${read ? "已读" : "未读"}`);
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  return (
    <div className="shell">
      <header className="topbar">
        <h1>Zflow Reader MVP</h1>
        <p>先把阅读器本体做扎实，再接评分与推荐</p>
      </header>

      <main className="grid">
        <section className="panel">
          <h2>连接设置</h2>
          <label htmlFor="apiBase">API Base URL</label>
          <div className="row">
            <input id="apiBase" value={apiBase} onChange={(e) => setApiBase(e.target.value)} />
            <button className="secondary" onClick={handleSaveAPIBase}>
              保存
            </button>
          </div>

          <h2>添加订阅</h2>
          <label htmlFor="feedUrl">RSS/Atom URL</label>
          <input
            id="feedUrl"
            value={feedURL}
            placeholder="https://example.com/feed.xml"
            onChange={(e) => setFeedURL(e.target.value)}
          />

          <div className="row">
            <button onClick={addFeed}>添加并首抓</button>
            <button className="secondary" onClick={loadFeeds}>
              刷新订阅
            </button>
            <button className="secondary" onClick={loadArticles}>
              刷新文章
            </button>
          </div>

          <h2>订阅源</h2>
          <div className="list">
            {feeds.length === 0 && <div className="item">暂无订阅</div>}
            {feeds.map((feed) => (
              <div key={feed.id} className="item">
                <div>
                  <strong>{feed.title || "(未命名源)"}</strong>
                </div>
                <div className="meta">
                  {feed.url} · items={feed.item_count} · {feed.last_fetch_status}
                </div>
              </div>
            ))}
          </div>

          <div className={`status ${error ? "error" : ""}`}>{error || status}</div>
        </section>

        <section className="panel">
          <h2>文章流</h2>
          <div className="list">
            {articles.length === 0 && <div className="item">暂无文章</div>}
            {articles.map((article) => (
              <button
                key={article.id}
                className={`item article ${selectedArticle?.id === article.id ? "active" : ""}`}
                onClick={() => selectArticle(article.id)}
              >
                <div>
                  <strong>{article.title || "(无标题)"}</strong>
                  <span className={`pill ${article.is_read ? "read" : "unread"}`}>{article.is_read ? "已读" : "未读"}</span>
                </div>
                <div className="meta">
                  feed={article.feed_id} · {article.published_at || article.created_at || "-"}
                </div>
              </button>
            ))}
          </div>

          <h2>文章详情</h2>
          <div className="detail">
            {!selectedArticle && <p>请选择一篇文章查看详情</p>}
            {selectedArticle && (
              <>
                <h3>{selectedArticle.title || "(无标题)"}</h3>
                <p className="meta">发布时间：{selectedArticle.published_at || "-"}</p>
                <p className="meta">
                  链接：
                  {selectedArticle.link ? (
                    <a href={selectedArticle.link} target="_blank" rel="noreferrer">
                      {selectedArticle.link}
                    </a>
                  ) : (
                    "-"
                  )}
                </p>
                <p className="meta">状态：{selectedArticle.is_read ? "已读" : "未读"}</p>
                <h4>摘要</h4>
                <p>{selectedArticle.summary || "(无摘要)"}</p>
                <div className="row">
                  <button onClick={() => markRead(true)}>标记已读</button>
                  <button className="secondary" onClick={() => markRead(false)}>
                    标记未读
                  </button>
                </div>
              </>
            )}
          </div>
        </section>
      </main>
    </div>
  );
}

