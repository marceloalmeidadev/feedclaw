<script setup lang="ts">
const { digest, loading, notFound, load } = useDigest()

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

const date = ref(todayISO())

function reload() {
  load(date.value)
}

onMounted(() => load(date.value))
</script>

<template>
  <div>
    <div class="flex flex-wrap items-end gap-3 mb-4">
      <h1 class="text-xl font-bold flex-1">Histórico</h1>
      <UFormField label="Data">
        <UInput v-model="date" type="date" @change="reload" />
      </UFormField>
      <UButton icon="i-lucide-search" color="primary" variant="soft" @click="reload">
        Ver digest
      </UButton>
    </div>

    <div v-if="loading" class="py-10 text-center text-muted">Carregando…</div>

    <EmptyState
      v-else-if="notFound || !digest"
      icon="i-lucide-calendar-x"
      title="Sem digest nesta data"
      description="Escolha outra data. Digests são gerados nos dias em que o agente rodou o fluxo."
    />

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
