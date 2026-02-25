import type { Article, Feed, Folder } from "./types";

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

  async createFeed(url: string, folderID?: number | null): Promise<Feed> {
    return this.request<Feed>("/api/v1/feeds", {
      method: "POST",
      body: JSON.stringify({ url, folder_id: folderID ?? null }),
    });
  }

  async listFolders(): Promise<Folder[]> {
    const data = await this.request<{ folders?: Folder[] }>("/api/v1/folders");
    return data.folders ?? [];
  }

  async createFolder(name: string, parentID?: number | null): Promise<Folder> {
    return this.request<Folder>("/api/v1/folders", {
      method: "POST",
      body: JSON.stringify({ name, parent_id: parentID ?? null }),
    });
  }

  async updateFolder(id: number, name: string, parentID?: number | null): Promise<Folder> {
    return this.request<Folder>(`/api/v1/folders/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ name, parent_id: parentID ?? null }),
    });
  }

  async deleteFolder(id: number): Promise<void> {
    await this.request(`/api/v1/folders/${id}`, { method: "DELETE" });
  }

  async listArticles(): Promise<Article[]> {
    const data = await this.request<{ articles?: Article[] }>("/api/v1/articles");
    return data.articles ?? [];
  }

  async updateFeedFolder(id: number, folderID: number | null): Promise<Feed> {
    return this.request<Feed>(`/api/v1/feeds/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ folder_id: folderID }),
    });
  }

  async deleteFeed(id: number): Promise<void> {
    await this.request(`/api/v1/feeds/${id}`, {
      method: "DELETE",
    });
  }

  async refreshFeed(id: number): Promise<void> {
    await this.request(`/api/v1/feeds/${id}/refresh`, {
      method: "POST",
    });
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
