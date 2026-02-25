export type Feed = {
  id: number;
  url: string;
  title: string;
  item_count: number;
  last_fetched_at: string;
  last_fetch_status: string;
  last_fetch_error?: string;
  created_at: string;
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

