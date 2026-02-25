export type Feed = {
  id: number;
  url: string;
  title: string;
  folder_id?: number | null;
  item_count: number;
  last_fetched_at: string;
  last_fetch_status: string;
  last_fetch_error?: string;
  created_at: string;
};

export type Folder = {
  id: number;
  name: string;
  parent_id?: number | null;
  created_at: string;
  updated_at: string;
};

export type Article = {
  id: number;
  feed_id: number;
  title: string;
  link: string;
  summary?: string;
  published_at?: string;
  is_read: boolean;
  created_at: string;
};
