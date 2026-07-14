<script setup lang="ts">
const api = useApi()
const { stats, refresh } = useStats()
const colorMode = useColorMode()
const toast = useToast()
const fetching = ref(false)

const links = [
  { label: 'Hoje', to: '/', icon: 'i-lucide-sun' },
  { label: 'Triagem', to: '/articles', icon: 'i-lucide-list' },
  { label: 'Feeds', to: '/feeds', icon: 'i-lucide-rss' },
  { label: 'Histórico', to: '/history', icon: 'i-lucide-history' },
]

const isDark = computed({
  get: () => colorMode.value === 'dark',
  set: (v: boolean) => { colorMode.preference = v ? 'dark' : 'light' },
})

function toggleDark() {
  isDark.value = !isDark.value
}

async function runFetch() {
  fetching.value = true
  try {
    await api.fetchNow()
    await refresh()
    toast.add({ title: 'Fetch concluído', color: 'success' })
  }
  finally {
    fetching.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <UApp>
    <div class="min-h-screen bg-default text-default">
      <header class="border-b border-default sticky top-0 z-10 bg-default/80 backdrop-blur">
        <div class="max-w-5xl mx-auto px-4 h-14 flex items-center gap-4">
          <NuxtLink to="/" class="font-bold text-lg flex items-center gap-2">
            <UIcon name="i-lucide-newspaper" class="text-primary" />
            FeedClaw
          </NuxtLink>

          <nav class="flex items-center gap-1 flex-1">
            <UButton
              v-for="l in links"
              :key="l.to"
              :to="l.to"
              :icon="l.icon"
              :label="l.label"
              color="neutral"
              variant="ghost"
              size="sm"
            />
          </nav>

          <UBadge v-if="stats" color="primary" variant="subtle">
            {{ stats.unread_total }} não lidos
          </UBadge>
          <UButton
            :loading="fetching"
            icon="i-lucide-refresh-cw"
            color="primary"
            variant="soft"
            size="sm"
            @click="runFetch"
          >
            Fetch
          </UButton>
          <UButton
            :icon="isDark ? 'i-lucide-moon' : 'i-lucide-sun'"
            color="neutral"
            variant="ghost"
            size="sm"
            aria-label="Alternar tema"
            @click="toggleDark"
          />
        </div>
      </header>

      <main class="max-w-5xl mx-auto px-4 py-6">
        <NuxtPage />
      </main>
    </div>
  </UApp>
</template>
