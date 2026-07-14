<script setup lang="ts">
import type { ArticleStatus } from '~/types'

const { articles, total, page, loading, filters, load, toggleRead, toggleStar, markRead } = useArticles()
const { stats, refresh: refreshStats } = useStats()

const statusItems: { label: string, value: ArticleStatus }[] = [
  { label: 'Não lidos', value: 'unread' },
  { label: 'Lidos', value: 'read' },
  { label: 'Starred', value: 'starred' },
  { label: 'Todos', value: 'all' },
]

// 'all' is a sentinel: Reka UI's Select forbids an empty-string item value, so
// we map it to "no category filter" when querying. Empty feed categories are
// omitted (they surface under "all").
const ALL_CATEGORIES = 'all'
const categorySel = ref(ALL_CATEGORIES)
const categoryItems = computed(() => [
  { label: 'Todas as categorias', value: ALL_CATEGORIES },
  ...(stats.value?.by_category ?? [])
    .filter(c => c.category !== '')
    .map(c => ({ label: `${c.category} (${c.unread})`, value: c.category })),
])

const activeIndex = ref(0)
const selected = ref<Set<number>>(new Set())

const activeArticle = computed(() => articles.value[activeIndex.value])

async function reload() {
  page.value = 1
  selected.value = new Set()
  activeIndex.value = 0
  await load()
}

watch(categorySel, (v) => {
  filters.category = v === ALL_CATEGORIES ? '' : v
})
watch(() => [filters.status, filters.category], reload)

function search() {
  reload()
}

function clampActive() {
  if (activeIndex.value >= articles.value.length) activeIndex.value = Math.max(0, articles.value.length - 1)
}

function openReader(id?: number) {
  const target = id ?? activeArticle.value?.id
  if (target) navigateTo(`/articles/${target}`)
}

function toggleSelect(id: number, checked: boolean) {
  if (checked) selected.value.add(id)
  else selected.value.delete(id)
  selected.value = new Set(selected.value)
}

async function markSelectedRead() {
  const ids = selected.value.size > 0 ? [...selected.value] : articles.value.map(a => a.id)
  await markRead(ids)
  selected.value = new Set()
  await refreshStats()
}

useKeyboardNav({
  next: () => { if (activeIndex.value < articles.value.length - 1) activeIndex.value++ },
  prev: () => { if (activeIndex.value > 0) activeIndex.value-- },
  toggleRead: async () => { if (activeArticle.value) { await toggleRead(activeArticle.value); await refreshStats() } },
  toggleStar: async () => { if (activeArticle.value) { await toggleStar(activeArticle.value); await refreshStats() } },
  open: () => openReader(),
  openOriginal: () => { if (activeArticle.value) window.open(activeArticle.value.url, '_blank') },
  markAllVisible: markSelectedRead,
})

async function goPage(delta: number) {
  const next = page.value + delta
  if (next < 1) return
  page.value = next
  await load()
  activeIndex.value = 0
}

onMounted(async () => {
  await Promise.all([load(), refreshStats()])
})

watch(articles, clampActive)
</script>

<template>
  <div>
    <div class="flex flex-wrap items-end gap-3 mb-4">
      <h1 class="text-xl font-bold flex-1">Triagem</h1>
      <USelect
        v-model="filters.status"
        :items="statusItems"
        value-key="value"
        class="w-40"
      />
      <USelect
        v-model="categorySel"
        :items="categoryItems"
        value-key="value"
        class="w-52"
      />
      <UInput
        v-model="filters.q"
        placeholder="Buscar (FTS)…"
        icon="i-lucide-search"
        class="w-56"
        @keydown.enter="search"
      />
    </div>

    <div class="flex items-center gap-2 mb-2 text-sm text-muted">
      <span>{{ total }} artigo(s)</span>
      <span v-if="selected.size">· {{ selected.size }} selecionado(s)</span>
      <div class="flex-1" />
      <UButton
        size="xs"
        color="primary"
        variant="soft"
        icon="i-lucide-check-check"
        @click="markSelectedRead"
      >
        {{ selected.size ? 'Marcar selecionados lidos' : 'Marcar visíveis lidos (Shift+A)' }}
      </UButton>
    </div>

    <div v-if="loading && !articles.length" class="py-10 text-center text-muted">
      Carregando…
    </div>
    <EmptyState
      v-else-if="!articles.length"
      icon="i-lucide-inbox"
      title="Nada por aqui"
      description="Sem artigos para este filtro. Rode um fetch ou ajuste os filtros."
    />

    <div v-else class="divide-y divide-default/60">
      <ArticleRow
        v-for="(a, i) in articles"
        :key="a.id"
        :article="a"
        :active="i === activeIndex"
        selectable
        :selected="selected.has(a.id)"
        @open="openReader(a.id)"
        @toggle-read="async () => { await toggleRead(a); await refreshStats() }"
        @toggle-star="async () => { await toggleStar(a); await refreshStats() }"
        @toggle-select="(c) => toggleSelect(a.id, c)"
      />
    </div>

    <div v-if="articles.length" class="flex items-center justify-between mt-4 text-sm">
      <UButton
        :disabled="page <= 1"
        icon="i-lucide-chevron-left"
        color="neutral"
        variant="ghost"
        size="sm"
        @click="goPage(-1)"
      >
        Anterior
      </UButton>
      <span class="text-muted">Página {{ page }}</span>
      <UButton
        :disabled="page * (filters.per_page ?? 50) >= total"
        icon="i-lucide-chevron-right"
        trailing
        color="neutral"
        variant="ghost"
        size="sm"
        @click="goPage(1)"
      >
        Próxima
      </UButton>
    </div>

    <p class="mt-6 text-xs text-muted">
      Atalhos: <kbd>j</kbd>/<kbd>k</kbd> navegar · <kbd>m</kbd> lido · <kbd>s</kbd> star ·
      <kbd>o</kbd>/<kbd>Enter</kbd> ler · <kbd>v</kbd> abrir original · <kbd>Shift</kbd>+<kbd>A</kbd> marcar visíveis
    </p>
  </div>
</template>
