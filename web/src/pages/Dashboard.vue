<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useAppsStore } from '@/stores/apps'

const apps = useAppsStore()
onMounted(() => apps.load())

const stats = computed(() => ({
  total: apps.items.length,
  running: apps.items.filter((a) => a.current_version).length,
  internal: apps.items.filter((a) => a.internal_only).length,
}))
</script>

<template>
  <div class="page">
    <h1>Dashboard</h1>
    <div class="cards">
      <div class="card">
        <div class="label">Apps</div>
        <div class="num">{{ stats.total }}</div>
      </div>
      <div class="card">
        <div class="label">Deployed</div>
        <div class="num">{{ stats.running }}</div>
      </div>
      <div class="card">
        <div class="label">Internal</div>
        <div class="num">{{ stats.internal }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page h1 { margin-top: 0; }
.cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; }
.card {
  padding: 1.25rem;
  border-radius: 1rem;
  background: var(--ui-bg-elevated);
  border: 1px solid var(--ui-border);
}
.label { color: var(--ui-text-muted); text-transform: uppercase; font-size: 0.75rem; letter-spacing: 0.05em; }
.num { font-size: 2.2rem; font-weight: 600; margin-top: 0.5rem; color: var(--color-primary-500); }
</style>
