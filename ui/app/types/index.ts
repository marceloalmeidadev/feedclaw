// Shapes mirror the engine's JSON (internal/store models + api responses).

export interface Feed {
  id: number
  url: string
  site_url?: string
  title: string
  category: string
  error_count: number
  disabled: boolean
  last_fetch_at?: string
  last_status?: number
  created_at: string
}

export interface Article {
  id: number
  feed_id: number
  url: string
  title: string
  summary?: string
  content?: string
  full_content?: string
  author?: string
  published_at?: string
  fetched_at: string
  read_at?: string | null
  starred: boolean
  feed_title?: string
  category?: string
}

export interface DigestTheme {
  id: number
  position: number
  name: string
  summary: string
  article_count: number
  articles?: Article[]
}

export interface Digest {
  id: number
  date: string
  generated_at: string
  model_note?: string
  themes: DigestTheme[]
}

export interface CategoryCount {
  category: string
  unread: number
}

export interface Stats {
  unread_total: number
  starred_total: number
  by_category: CategoryCount[]
}

export type ArticleStatus = 'unread' | 'read' | 'starred' | 'all'

export interface ArticleQuery {
  status?: ArticleStatus
  category?: string
  theme?: number
  q?: string
  page?: number
  per_page?: number
}

export interface ArticlePage {
  articles: Article[]
  total: number
  page: number
}
