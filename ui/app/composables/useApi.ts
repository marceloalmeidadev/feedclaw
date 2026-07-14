import type { Article, ArticlePage, ArticleQuery, Digest, Feed, Stats } from '~/types'

// useApi wraps $fetch with a single place for error toasts. All calls are
// same-origin relative paths (/api/...): in dev they are proxied to the engine,
// in production the engine serves both the UI and the API.
export function useApi() {
  const toast = useToast()

  async function request<T>(url: string, opts?: Parameters<typeof $fetch>[1]): Promise<T> {
    try {
      return await $fetch<T>(url, opts)
    }
    catch (e: unknown) {
      const err = e as { data?: { error?: { message?: string } }, message?: string }
      const message = err?.data?.error?.message ?? err?.message ?? 'Erro na requisição'
      toast.add({ title: 'Erro', description: message, color: 'error' })
      throw e
    }
  }

  return {
    listArticles: (query: ArticleQuery) => request<ArticlePage>('/api/articles', { query }),
    getArticle: (id: number) => request<Article>(`/api/articles/${id}`),
    fullArticle: (id: number) => request<Article>(`/api/articles/${id}/full`, { method: 'POST' }),
    setRead: (ids: number[], read: boolean) =>
      request<{ affected: number }>('/api/articles/read', { method: 'PATCH', body: { ids, read } }),
    setStar: (ids: number[], starred: boolean) =>
      request<{ affected: number }>('/api/articles/star', { method: 'PATCH', body: { ids, starred } }),

    listFeeds: () => request<{ feeds: Feed[] }>('/api/feeds'),
    addFeed: (url: string, category: string) =>
      request<{ feed: Feed, created: boolean }>('/api/feeds', { method: 'POST', body: { url, category } }),
    deleteFeed: (id: number) => request<{ removed: number }>(`/api/feeds/${id}`, { method: 'DELETE' }),
    importOpml: (xml: string) =>
      request<{ total: number, added: number, existing: number }>('/api/feeds/import', {
        method: 'POST',
        body: xml,
        headers: { 'Content-Type': 'text/xml' },
      }),
    fetchNow: () => request<{ status: string }>('/api/fetch', { method: 'POST' }),

    getDigest: (date?: string) => request<Digest>('/api/digests', { query: date ? { date } : {} }),
    themeArticles: (date: string, themeId: number) =>
      request<{ articles: Article[] }>(`/api/digests/${date}/themes/${themeId}/articles`),

    stats: () => request<Stats>('/api/stats'),
  }
}
