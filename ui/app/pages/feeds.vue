<script setup lang="ts">
import type { Feed } from '~/types'

const api = useApi()
const toast = useToast()

const feeds = ref<Feed[]>([])
const loading = ref(false)
const newUrl = ref('')
const newCategory = ref('')
const adding = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  try {
    const res = await api.listFeeds()
    feeds.value = res.feeds ?? []
  }
  finally {
    loading.value = false
  }
}

async function addFeed() {
  if (!newUrl.value.trim()) return
  adding.value = true
  try {
    await api.addFeed(newUrl.value.trim(), newCategory.value.trim())
    newUrl.value = ''
    newCategory.value = ''
    await load()
  }
  finally {
    adding.value = false
  }
}

async function removeFeed(feed: Feed) {
  await api.deleteFeed(feed.id)
  await load()
}

function pickFile() {
  fileInput.value?.click()
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const xml = await file.text()
  const res = await api.importOpml(xml)
  toast.add({ title: 'OPML importado', description: `${res.added} novos, ${res.existing} já existentes`, color: 'success' })
  input.value = ''
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center gap-2 mb-4">
      <h1 class="text-xl font-bold flex-1">Feeds</h1>
      <UButton icon="i-lucide-upload" color="neutral" variant="soft" size="sm" @click="pickFile">
        Importar OPML
      </UButton>
      <input ref="fileInput" type="file" accept=".opml,.xml,text/xml" class="hidden" @change="onFile">
    </div>

    <UCard class="mb-4">
      <form class="flex flex-wrap items-end gap-2" @submit.prevent="addFeed">
        <UFormField label="URL do feed" class="flex-1 min-w-56">
          <UInput v-model="newUrl" placeholder="https://exemplo.com/feed.xml" class="w-full" />
        </UFormField>
        <UFormField label="Categoria">
          <UInput v-model="newCategory" placeholder="Go" />
        </UFormField>
        <UButton type="submit" :loading="adding" icon="i-lucide-plus" color="primary">
          Adicionar
        </UButton>
      </form>
    </UCard>

    <div v-if="loading" class="py-10 text-center text-muted">Carregando…</div>
    <EmptyState
      v-else-if="!feeds.length"
      icon="i-lucide-rss"
      title="Nenhum feed"
      description="Adicione um feed por URL ou importe um export OPML do Feedly."
    />

    <div v-else class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="text-left text-muted border-b border-default">
          <tr>
            <th class="py-2 pr-3 font-medium">Título</th>
            <th class="py-2 px-3 font-medium">Categoria</th>
            <th class="py-2 px-3 font-medium">Último fetch</th>
            <th class="py-2 px-3 font-medium">Status</th>
            <th class="py-2 pl-3" />
          </tr>
        </thead>
        <tbody class="divide-y divide-default/60">
          <tr v-for="feed in feeds" :key="feed.id" class="align-middle">
            <td class="py-2 pr-3 max-w-xs">
              <p class="truncate font-medium">{{ feed.title }}</p>
              <a :href="feed.url" target="_blank" class="truncate block text-xs text-muted hover:text-primary">
                {{ feed.url }}
              </a>
            </td>
            <td class="py-2 px-3">{{ feed.category || '—' }}</td>
            <td class="py-2 px-3 text-muted">{{ formatRelative(feed.last_fetch_at) }}</td>
            <td class="py-2 px-3">
              <UBadge v-if="feed.error_count >= 10" color="error" variant="subtle" size="sm">
                {{ feed.error_count }} erros
              </UBadge>
              <UBadge v-else-if="feed.error_count > 0" color="warning" variant="subtle" size="sm">
                {{ feed.error_count }} erros
              </UBadge>
              <UBadge v-else color="success" variant="subtle" size="sm">ok</UBadge>
            </td>
            <td class="py-2 pl-3 text-right">
              <UButton
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="xs"
                aria-label="Remover"
                @click="removeFeed(feed)"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
