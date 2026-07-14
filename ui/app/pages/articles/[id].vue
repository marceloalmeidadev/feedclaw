<script setup lang="ts">
import type { Article } from '~/types'

const route = useRoute()
const api = useApi()
const { refresh: refreshStats } = useStats()

const id = computed(() => Number(route.params.id))
const article = ref<Article | null>(null)
const loading = ref(true)
const extracting = ref(false)

// Sanitize feed HTML before rendering — untrusted input (XSS defense).
const safeHtml = computed(() => sanitizeHtml(article.value?.full_content || article.value?.content))
const hasBody = computed(() => !!(article.value?.full_content || article.value?.content))

async function loadArticle() {
  loading.value = true
  try {
    article.value = await api.getArticle(id.value)
  }
  finally {
    loading.value = false
  }
}

async function loadFull() {
  if (!article.value) return
  extracting.value = true
  try {
    article.value = await api.fullArticle(id.value)
  }
  finally {
    extracting.value = false
  }
}

async function toggleRead() {
  if (!article.value) return
  const read = !article.value.read_at
  await api.setRead([article.value.id], read)
  article.value.read_at = read ? new Date().toISOString() : null
  await refreshStats()
}

async function toggleStar() {
  if (!article.value) return
  const starred = !article.value.starred
  await api.setStar([article.value.id], starred)
  article.value.starred = starred
}

onMounted(loadArticle)
</script>

<template>
  <div class="max-w-2xl mx-auto">
    <UButton
      to="/articles"
      icon="i-lucide-arrow-left"
      color="neutral"
      variant="ghost"
      size="sm"
      class="mb-4"
    >
      Voltar
    </UButton>

    <div v-if="loading" class="py-10 text-center text-muted">Carregando…</div>

    <article v-else-if="article">
      <h1 class="text-2xl font-bold leading-tight mb-2">{{ article.title }}</h1>
      <p class="text-sm text-muted mb-4">
        {{ article.feed_title }}
        <span v-if="article.author"> · {{ article.author }}</span>
        · {{ formatRelative(article.published_at) }}
      </p>

      <div class="flex flex-wrap items-center gap-2 mb-6">
        <UButton
          :icon="article.read_at ? 'i-lucide-circle' : 'i-lucide-circle-check'"
          :label="article.read_at ? 'Marcar não lido' : 'Marcar lido'"
          color="neutral"
          variant="soft"
          size="sm"
          @click="toggleRead"
        />
        <UButton
          :icon="'i-lucide-star'"
          :label="article.starred ? 'Remover star' : 'Ler depois'"
          :color="article.starred ? 'warning' : 'neutral'"
          variant="soft"
          size="sm"
          @click="toggleStar"
        />
        <UButton
          :to="article.url"
          target="_blank"
          icon="i-lucide-external-link"
          label="Abrir original"
          color="neutral"
          variant="soft"
          size="sm"
        />
      </div>

      <!-- Content is sanitized with DOMPurify (see sanitizeHtml); safe to render. -->
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-if="hasBody" class="reader-content" v-html="safeHtml" />

      <EmptyState
        v-else
        icon="i-lucide-file-text"
        title="Sem conteúdo cacheado"
        description="Este artigo ainda não tem o conteúdo completo extraído."
      >
        <UButton
          :loading="extracting"
          icon="i-lucide-download"
          color="primary"
          @click="loadFull"
        >
          Carregar artigo completo
        </UButton>
      </EmptyState>

      <div v-if="hasBody && !article.full_content" class="mt-6">
        <UButton
          :loading="extracting"
          icon="i-lucide-sparkles"
          color="neutral"
          variant="soft"
          size="sm"
          @click="loadFull"
        >
          Extrair conteúdo completo (Reader Mode)
        </UButton>
      </div>
    </article>
  </div>
</template>

<style scoped>
.reader-content {
  line-height: 1.7;
  font-size: 1.05rem;
}
.reader-content :deep(p) {
  margin: 0 0 1rem;
}
.reader-content :deep(h1),
.reader-content :deep(h2),
.reader-content :deep(h3) {
  font-weight: 600;
  margin: 1.6rem 0 0.6rem;
  line-height: 1.3;
}
.reader-content :deep(a) {
  color: var(--ui-primary);
  text-decoration: underline;
}
.reader-content :deep(img) {
  max-width: 100%;
  height: auto;
  border-radius: 0.5rem;
  margin: 1rem 0;
}
.reader-content :deep(pre) {
  overflow-x: auto;
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
  background: var(--ui-bg-elevated);
  margin: 1rem 0;
}
.reader-content :deep(blockquote) {
  border-left: 3px solid var(--ui-border);
  padding-left: 1rem;
  color: var(--ui-text-muted);
  margin: 1rem 0;
}
.reader-content :deep(ul),
.reader-content :deep(ol) {
  margin: 0 0 1rem 1.25rem;
  list-style: revert;
}
</style>
