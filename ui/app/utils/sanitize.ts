import DOMPurify from 'dompurify'

// sanitizeHtml cleans feed-provided HTML before it is rendered with v-html.
// Feed content is untrusted input — this is the XSS defense required by the spec.
// Runs client-side only (the app is SPA, ssr: false).
export function sanitizeHtml(html: string | undefined | null): string {
  if (!html) return ''
  if (import.meta.server) return ''
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    ADD_ATTR: ['target', 'rel'],
    FORBID_TAGS: ['style', 'form', 'input', 'button'],
    FORBID_ATTR: ['onerror', 'onload', 'onclick'],
  })
}
