<script setup lang="ts">
import type { Article } from '~/types'

const props = defineProps<{
  article: Article
  active?: boolean
  selectable?: boolean
  selected?: boolean
}>()

const emit = defineEmits<{
  open: []
  toggleRead: []
  toggleStar: []
  toggleSelect: [checked: boolean]
}>()

const isRead = computed(() => !!props.article.read_at)
</script>

<template>
  <div
    class="group flex items-center gap-3 px-3 py-2 rounded-md border border-transparent cursor-pointer"
    :class="[
      active ? 'bg-elevated border-default' : 'hover:bg-elevated/60',
      isRead ? 'opacity-60' : '',
    ]"
    @click="emit('open')"
  >
    <UCheckbox
      v-if="selectable"
      :model-value="selected"
      aria-label="Selecionar"
      @update:model-value="(v: boolean | 'indeterminate') => emit('toggleSelect', v === true)"
      @click.stop
    />

    <span
      class="size-2 rounded-full shrink-0"
      :class="isRead ? 'bg-transparent' : 'bg-primary'"
      :title="isRead ? 'lido' : 'não lido'"
    />

    <div class="min-w-0 flex-1">
      <p class="truncate text-sm" :class="isRead ? 'font-normal' : 'font-medium'">
        {{ article.title }}
      </p>
      <p class="truncate text-xs text-muted">
        {{ article.feed_title }} · {{ formatRelative(article.published_at) }}
        <span v-if="article.category"> · {{ article.category }}</span>
      </p>
    </div>

    <div class="flex items-center gap-1 shrink-0" @click.stop>
      <UButton
        :icon="article.starred ? 'i-lucide-star' : 'i-lucide-star'"
        :color="article.starred ? 'warning' : 'neutral'"
        variant="ghost"
        size="xs"
        :aria-label="article.starred ? 'Remover star' : 'Star'"
        @click="emit('toggleStar')"
      />
      <UButton
        :icon="isRead ? 'i-lucide-circle' : 'i-lucide-circle-check'"
        color="neutral"
        variant="ghost"
        size="xs"
        :aria-label="isRead ? 'Marcar não lido' : 'Marcar lido'"
        @click="emit('toggleRead')"
      />
      <UButton
        icon="i-lucide-external-link"
        color="neutral"
        variant="ghost"
        size="xs"
        :to="article.url"
        target="_blank"
        aria-label="Abrir original"
      />
    </div>
  </div>
</template>
