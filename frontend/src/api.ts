import type { Article, Feed } from "./types";

export class ApiClient {
  constructor(private readonly baseURL: string) {}

  private buildURL(path: string): string {
    return `${this.baseURL.replace(/\/$/, "")}${path}`;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(this.buildURL(path), {
      headers: { "Content-Type": "application/json" },
      ...options,
    });
    const data = (await response.json().catch(() => ({}))) as Record<string, unknown>;
    if (!response.ok) {
      throw new Error((data.error as string) || `HTTP ${response.status}`);
    }
    return data as T;
  }

  async listFeeds(): Promise<Feed[]> {
    const data = await this.request<{ feeds?: Feed[] }>("/api/v1/feeds");
    return data.feeds ?? [];
  }

  async createFeed(url: string): Promise<Feed> {
    return this.request<Feed>("/api/v1/feeds", {
      method: "POST",
      body: JSON.stringify({ url }),
    });
  }

  async listArticles(): Promise<Article[]> {
    const data = await this.request<{ articles?: Article[] }>("/api/v1/articles");
    return data.articles ?? [];
  }

  async getArticle(id: number): Promise<Article> {
    return this.request<Article>(`/api/v1/articles/${id}`);
  }

  async setArticleRead(id: number, read: boolean): Promise<Article> {
    return this.request<Article>(`/api/v1/articles/${id}/read`, {
      method: "PATCH",
      body: JSON.stringify({ read }),
    });
  }
}

