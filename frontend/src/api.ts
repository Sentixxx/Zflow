import type { Article, Feed, Folder } from "./types";
import { createLogger } from "./lib/logger";

const apiLogger = createLogger("api");

export class ApiClient {
  constructor(private readonly baseURL: string) {}

  private buildURL(path: string): string {
    return `${this.baseURL.replace(/\/$/, "")}${path}`;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const startedAt = Date.now();
    apiLogger.debug("request:start", { method: options.method || "GET", path });
    try {
      const response = await fetch(this.buildURL(path), {
        headers: { "Content-Type": "application/json" },
        ...options,
      });
      const data = (await response.json().catch(() => ({}))) as Record<string, unknown>;
      const durationMs = Date.now() - startedAt;
      if (!response.ok) {
        apiLogger.warn("request:failed", {
          method: options.method || "GET",
          path,
          status_code: response.status,
          duration_ms: durationMs,
        });
        throw new Error((data.error as string) || `HTTP ${response.status}`);
      }
      apiLogger.debug("request:ok", {
        method: options.method || "GET",
        path,
        status_code: response.status,
        duration_ms: durationMs,
      });
      return data as T;
    } catch (error) {
      apiLogger.error("request:error", {
        method: options.method || "GET",
        path,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
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

  async updateFeedScript(id: number, script: string, scriptLang: "shell" | "python" | "javascript"): Promise<Feed> {
    return this.request<Feed>(`/api/v1/feeds/${id}/script`, {
      method: "PATCH",
      body: JSON.stringify({ script, script_lang: scriptLang }),
    });
  }

  async updateFeedTitle(id: number, title: string): Promise<Feed> {
    return this.request<Feed>(`/api/v1/feeds/${id}/title`, {
      method: "PATCH",
      body: JSON.stringify({ title }),
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

  async exportProfile(): Promise<Blob> {
    const response = await fetch(this.buildURL("/api/v1/data/export/profile"));
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    return response.blob();
  }

  async importProfile(rawJSON: string): Promise<{ imported_feeds?: number; updated_feeds?: number; imported_folders?: number }> {
    return this.request<{ imported_feeds?: number; updated_feeds?: number; imported_folders?: number }>("/api/v1/data/import/profile", {
      method: "POST",
      body: rawJSON,
    });
  }

  async exportOPML(): Promise<Blob> {
    const response = await fetch(this.buildURL("/api/v1/data/export/opml"));
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    return response.blob();
  }

  async importOPML(rawOPML: string): Promise<{ imported_feeds?: number; updated_feeds?: number; imported_folders?: number }> {
    const response = await fetch(this.buildURL("/api/v1/data/import/opml"), {
      method: "POST",
      headers: { "Content-Type": "text/xml; charset=utf-8" },
      body: rawOPML,
    });
    const data = (await response.json().catch(() => ({}))) as Record<string, unknown>;
    if (!response.ok) {
      throw new Error((data.error as string) || `HTTP ${response.status}`);
    }
    return data as { imported_feeds?: number; updated_feeds?: number; imported_folders?: number };
  }

  async getNetworkSettings(): Promise<{ proxy_url?: string }> {
    return this.request<{ proxy_url?: string }>("/api/v1/settings/network");
  }

  async updateNetworkSettings(proxyURL: string): Promise<{ proxy_url?: string }> {
    return this.request<{ proxy_url?: string }>("/api/v1/settings/network", {
      method: "PATCH",
      body: JSON.stringify({ proxy_url: proxyURL }),
    });
  }
}
