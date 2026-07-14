import type { Digest } from '~/types'

// useDigest loads a digest by date (or the most recent when date is omitted).
export function useDigest() {
  const api = useApi()
  const digest = ref<Digest | null>(null)
  const loading = ref(false)
  const notFound = ref(false)

  async function load(date?: string) {
    loading.value = true
    notFound.value = false
    try {
      digest.value = await api.getDigest(date)
    }
    catch {
      digest.value = null
      notFound.value = true
    }
    finally {
      loading.value = false
    }
  }

  return { digest, loading, notFound, load }
}
