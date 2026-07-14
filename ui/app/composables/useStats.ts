import type { Stats } from '~/types'

// useStats holds the badge counters shared by the nav and pages.
export function useStats() {
  const api = useApi()
  const stats = useState<Stats | null>('stats', () => null)

  async function refresh() {
    try {
      stats.value = await api.stats()
    }
    catch {
      // errors already surfaced by useApi's toast
    }
  }

  return { stats, refresh }
}
