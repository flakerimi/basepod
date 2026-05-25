<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useAppsStore } from '@/stores/apps'

const apps = useAppsStore()
onMounted(() => apps.load())

const stats = computed(() => [
  { label: 'Apps',     value: apps.items.length,                         icon: 'i-lucide-boxes' },
  { label: 'Deployed', value: apps.items.filter(a => a.current_version).length, icon: 'i-lucide-rocket' },
  { label: 'Internal', value: apps.items.filter(a => a.internal_only).length,   icon: 'i-lucide-lock' },
])
</script>

<template>
  <div class="page">
    <header class="head">
      <h1>Dashboard</h1>
      <p class="muted">Overview of services running on this host.</p>
    </header>

    <div class="cards">
      <UCard v-for="s in stats" :key="s.label" :ui="{ body: 'p-5' }">
        <div class="stat">
          <UIcon :name="s.icon" class="ico" />
          <div>
            <div class="label">{{ s.label }}</div>
            <div class="num">{{ s.value }}</div>
          </div>
        </div>
      </UCard>
    </div>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 1.5rem; }
.head h1 { margin: 0 0 0.25rem; font-size: 1.5rem; }
.muted { color: var(--ui-text-muted); margin: 0; }
.cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; }
.stat { display: flex; gap: 0.9rem; align-items: center; }
.ico {
  font-size: 1.5rem;
  color: var(--ui-primary);
  width: 2.5rem; height: 2.5rem;
  display: grid; place-items: center;
  background: color-mix(in oklch, var(--ui-primary) 12%, transparent);
  border-radius: 0.75rem;
}
.label { color: var(--ui-text-muted); font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; }
.num { font-size: 1.8rem; font-weight: 600; line-height: 1.1; color: var(--ui-text-highlighted); }
</style>
