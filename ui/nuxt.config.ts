// FeedClaw UI — SPA (ssr: false) generated with `nuxt generate` and embedded in
// the Go binary (Phase 7). In dev it proxies /api to the local engine.
export default defineNuxtConfig({
  ssr: false,
  devtools: { enabled: true },
  modules: ['@nuxt/ui', '@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      title: 'FeedClaw',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
    },
  },
  nitro: {
    // Dev-only: forward API calls to the running engine (feedclaw serve).
    // `nuxt generate` produces the static SPA output automatically (ssr: false).
    devProxy: {
      '/api': {
        target: 'http://127.0.0.1:8484/api',
        changeOrigin: true,
      },
    },
  },
  compatibilityDate: '2025-01-01',
})
