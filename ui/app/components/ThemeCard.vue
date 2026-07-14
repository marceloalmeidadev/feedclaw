<script setup lang="ts">
import type { Article, DigestTheme } from '~/types'

const props = defineProps<{
  theme: DigestTheme
  date: string
}>()

const api = useApi()
const { refresh: refreshStats } = useStats()

const expanded = ref(false)
const articles = ref<Article[]>([])
const loaded = ref(false)
const loading = ref(false)

async function toggle() {
  expanded.value = !expanded.value
  if (expanded.value && !loaded.value) {
    loading.value = true
    try {
      const res = await api.themeArticles(props.date, props.theme.id)
      articles.value = res.articles ?? []
      loaded.value = true
    }
    finally {
      loading.value = false
    }
  }
}

const unreadCount = computed(() => articles.value.filter(a => !a.read_at).length)

async function markAllRead() {
  if (!loaded.value) {
    const res = await api.themeArticles(props.date, props.theme.id)
    articles.value = res.articles ?? []
    loaded.value = true
  }
  const ids = articles.value.filter(a => !a.read_at).map(a => a.id)
  if (ids.length) {
    await api.setRead(ids, true)
    const now = new Date().toISOString()
    for (const a of articles.value) if (ids.includes(a.id)) a.read_at = now
    await refreshStats()
  }
}

function openAll() {
  for (const a of articles.value) window.open(a.url, '_blank')
}
</script>

<template>
  <UCard>
    <div class="flex items-start gap-3">
      <button class="min-w-0 flex-1 text-left" @click="toggle">
        <div class="flex items-center gap-2">
          <UIcon
            :name="expanded ? 'i-lucide-chevron-down' : 'i-lucide-chevron-right'"
            class="text-muted shrink-0"
          />
          <h3 class="font-semibold truncate">{{ theme.name }}</h3>
          <UBadge color="neutral" variant="subtle" size="sm">{{ theme.article_count }}</UBadge>
          <UBadge v-if="loaded && unreadCount" color="primary" variant="subtle" size="sm">
            {{ unreadCount }} não lidos
          </UBadge>
        </div>
        <p v-if="theme.summary" class="text-sm text-muted mt-1 ml-6">{{ theme.summary }}</p>
      </button>
    </div>

    <template v-if="expanded">
      <div class="mt-3 flex items-center gap-2">
        <UButton
          size="xs"
          color="primary"
          variant="soft"
          icon="i-lucide-check-check"
          @click="markAllRead"
        >
          Marcar tudo como lido
        </UButton>
        <UButton
          v-if="articles.length"
          size="xs"
          color="neutral"
          variant="soft"
          icon="i-lucide-external-link"
          @click="openAll"
        >
          Abrir todos os links
        </UButton>
      </div>

      <div v-if="loading" class="py-4 text-center text-sm text-muted">Carregando…</div>
      <ul v-else class="mt-2 divide-y divide-default/60">
        <li v-for="a in articles" :key="a.id" class="py-2 flex items-center gap-2">
          <span class="size-1.5 rounded-full shrink-0" :class="a.read_at ? 'bg-transparent' : 'bg-primary'" />
          <NuxtLink :to="`/articles/${a.id}`" class="text-sm truncate flex-1 hover:underline">
            {{ a.title }}
          </NuxtLink>
          <span class="text-xs text-muted shrink-0">{{ formatRelative(a.published_at) }}</span>
          <a :href="a.url" target="_blank" class="text-muted hover:text-primary shrink-0" aria-label="Original">
            <UIcon name="i-lucide-external-link" class="size-4" />
          </a>
        </li>
      </ul>
    </template>
  </UCard>
</template>
