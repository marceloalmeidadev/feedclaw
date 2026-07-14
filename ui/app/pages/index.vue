<script setup lang="ts">
const { digest, loading, notFound, load } = useDigest()
const api = useApi()
const { refresh: refreshStats } = useStats()
const fetching = ref(false)

async function runFetch() {
  fetching.value = true
  try {
    await api.fetchNow()
    await Promise.all([load(), refreshStats()])
  }
  finally {
    fetching.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex items-center gap-2 mb-4">
      <h1 class="text-xl font-bold flex-1">Hoje</h1>
      <UButton
        v-if="digest"
        to="/history"
        icon="i-lucide-history"
        color="neutral"
        variant="ghost"
        size="sm"
      >
        Histórico
      </UButton>
    </div>

    <div v-if="loading" class="py-10 text-center text-muted">Carregando digest…</div>

    <EmptyState
      v-else-if="notFound || !digest"
      icon="i-lucide-sparkles"
      title="Nenhum digest ainda"
      description="Rode um fetch e peça ao agente para gerar o digest do dia, ou faça a triagem manualmente."
    >
      <div class="flex gap-2">
        <UButton :loading="fetching" icon="i-lucide-refresh-cw" color="primary" @click="runFetch">
          Rodar fetch
        </UButton>
        <UButton to="/articles" icon="i-lucide-list" color="neutral" variant="soft">
          Ir para a triagem
        </UButton>
      </div>
    </EmptyState>

    <template v-else>
      <p class="text-sm text-muted mb-4">
        Digest de {{ digest.date }} · {{ digest.themes.length }} temas
      </p>
      <div class="grid gap-3">
        <ThemeCard
          v-for="theme in digest.themes"
          :key="theme.id"
          :theme="theme"
          :date="digest.date"
        />
      </div>
    </template>
  </div>
</template>
