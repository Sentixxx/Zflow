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
