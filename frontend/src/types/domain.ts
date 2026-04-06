export interface Feed {
  id: number;
  url: string;
  title: string;
  folder_id?: number | null;
  custom_script?: string;
  custom_script_lang?: "shell" | "python" | "javascript";
  icon_url?: string;
  item_count: number;
  last_fetched_at: string;
  last_fetch_status: string;
  last_fetch_error?: string;
  created_at: string;
}

export interface Folder {
  id: number;
  name: string;
  parent_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface RecommendationScores {
  quality: number;
  relevance: number;
  novelty: number;
  composite: number;
}

export interface ArticleSourceField {
  key: string;
  value?: string;
  value_html?: string;
}

export interface ArticleSourcePayload {
  feed_type?: string;
  title?: string;
  link?: string;
  summary?: string;
  published_at?: string;
  fields?: ArticleSourceField[];
}

export interface TopicCluster {
  id: number;
  title: string;
  summary: string;
  article_count: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface TopicClusterMember {
  cluster_id: number;
  article_id: number;
  similarity: number;
  is_representative: boolean;
  added_at: string;
}

export interface TopicBrief {
  id: number;
  title: string;
  slug: string;
  content: string;
  level: "daily" | "weekly" | "monthly";
  period_start: string;
  period_end: string;
  source_cluster_ids: number[];
  source_article_ids: number[];
  parent_brief_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface InterestProfile {
  id: number;
  label: string;
  weight: number;
  source_article_ids: number[];
  last_reinforced_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AgentRun {
  id: number;
  agent_type: string;
  status: "running" | "completed" | "failed";
  input_summary: string;
  output_summary: string;
  items_processed: number;
  items_created: number;
  error?: string;
  started_at: string;
  completed_at?: string | null;
}

export interface Article {
  id: number;
  feed_id: number;
  title: string;
  link: string;
  summary?: string;
  ai_summary?: string;
  ai_summary_status?: string;
  ai_summary_updated_at?: string;
  display_summary?: string;
  display_summary_status?: string;
  display_summary_updated_at?: string;
  full_content?: string;
  source_payload?: ArticleSourcePayload;
  cover_url?: string;
  published_at?: string;
  is_read: boolean;
  is_favorite: boolean;
  favorited_at?: string;
  created_at: string;
  summary_debug?: {
    strategy?: string;
    query_mode?: string;
    chunk_count?: number;
    window_count?: number;
    rewrite_passed?: boolean;
    final_sentence_closed?: boolean;
    used_ai?: boolean;
  };
  recommendation_scores?: RecommendationScores;
}
