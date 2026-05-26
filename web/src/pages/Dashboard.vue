<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useAppsStore } from '@/stores/apps'

const apps = useAppsStore()
onMounted(() => apps.load())

const stats = computed(() => [
  { label: 'Apps',     value: apps.items.length,                                  icon: 'i-lucide-boxes' },
  { label: 'Deployed', value: apps.items.filter(a => a.current_version).length,   icon: 'i-lucide-rocket' },
  { label: 'Internal', value: apps.items.filter(a => a.internal_only).length,     icon: 'i-lucide-lock' },
])
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <h1 class="m-0 mb-1 text-2xl font-semibold">Dashboard</h1>
      <p class="m-0 text-(--ui-text-muted)">Overview of services running on this host.</p>
    </header>

    <div class="grid grid-cols-3 gap-4">
      <UCard v-for="s in stats" :key="s.label" :ui="{ body: 'p-5' }">
        <div class="flex items-center gap-3.5">
          <div class="grid size-10 place-items-center rounded-xl bg-(--ui-primary)/15 text-2xl text-(--ui-primary)">
            <UIcon :name="s.icon" />
          </div>
          <div>
            <div class="text-xs uppercase tracking-wider text-(--ui-text-muted)">{{ s.label }}</div>
            <div class="text-3xl font-semibold leading-tight text-(--ui-text-highlighted)">{{ s.value }}</div>
          </div>
        </div>
      </UCard>
    </div>
  </div>
</template>
