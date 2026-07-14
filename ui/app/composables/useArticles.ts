import type { Article, ArticleQuery } from '~/types'

// useArticles manages the triage list: filters, pagination and optimistic
// read/star mutations (the UI updates first and rolls back on error).
export function useArticles() {
  const api = useApi()

  const articles = ref<Article[]>([])
  const total = ref(0)
  const page = ref(1)
  const loading = ref(false)
  const filters = reactive<ArticleQuery>({ status: 'unread', per_page: 50 })

  async function load() {
    loading.value = true
    try {
      const res = await api.listArticles({ ...filters, page: page.value })
      articles.value = res.articles ?? []
      total.value = res.total
    }
    finally {
      loading.value = false
    }
  }

  async function toggleRead(a: Article) {
    const previous = a.read_at ?? null
    const nextRead = !a.read_at
    a.read_at = nextRead ? new Date().toISOString() : null
    try {
      await api.setRead([a.id], nextRead)
    }
    catch {
      a.read_at = previous
    }
  }

  async function toggleStar(a: Article) {
    const previous = a.starred
    a.starred = !previous
    try {
      await api.setStar([a.id], a.starred)
    }
    catch {
      a.starred = previous
    }
  }

  async function markRead(ids: number[]) {
    if (ids.length === 0) return
    const now = new Date().toISOString()
    const snapshot = new Map(articles.value.map(a => [a.id, a.read_at ?? null]))
    for (const a of articles.value) {
      if (ids.includes(a.id)) a.read_at = now
    }
    try {
      await api.setRead(ids, true)
    }
    catch {
      for (const a of articles.value) {
        if (snapshot.has(a.id)) a.read_at = snapshot.get(a.id) ?? null
      }
    }
  }

  return { articles, total, page, loading, filters, load, toggleRead, toggleStar, markRead }
}
