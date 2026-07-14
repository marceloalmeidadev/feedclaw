<script setup lang="ts">
defineProps<{
  icon: string
  title: string
  description?: string
}>()
</script>

<template>
  <div class="py-12 flex flex-col items-center text-center gap-3">
    <UIcon :name="icon" class="size-10 text-muted" />
    <div>
      <p class="font-medium">{{ title }}</p>
      <p v-if="description" class="text-sm text-muted mt-1 max-w-md">{{ description }}</p>
    </div>
    <slot />
  </div>
</template>
