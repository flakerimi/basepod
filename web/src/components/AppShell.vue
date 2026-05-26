<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Logo from '@/components/Logo.vue'

const router = useRouter()
const auth = useAuthStore()

const items = computed(() => [
  [
    { label: 'Dashboard', icon: 'i-lucide-layout-dashboard', to: '/' },
    { label: 'Apps',       icon: 'i-lucide-boxes',           to: '/apps' },
    { label: 'Templates',  icon: 'i-lucide-shapes',          to: '/templates' },
    { label: 'Settings',   icon: 'i-lucide-settings',        to: '/settings' },
  ],
])

const initials = computed(() => (auth.user?.username ?? '?').slice(0, 2).toUpperCase())

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="grid min-h-screen grid-cols-[240px_1fr] bg-(--ui-bg)">
    <aside class="flex flex-col gap-3 overflow-hidden border-r border-(--ui-border) bg-(--ui-bg-muted) px-3 py-4">
      <div class="px-2 pb-4 pt-2"><Logo :height="26" /></div>
      <UNavigationMenu orientation="vertical" :items="items" class="w-full" />
      <div class="flex-1" />
      <div class="flex items-center gap-2 border-t border-(--ui-border) px-2 pb-1 pt-3">
        <UAvatar :alt="initials" size="sm" />
        <div class="flex flex-col leading-tight">
          <div class="text-sm text-(--ui-text-highlighted)">{{ auth.user?.username }}</div>
          <button
            class="cursor-pointer border-0 bg-transparent p-0 text-left text-xs text-(--ui-text-muted) hover:text-(--ui-primary)"
            @click="logout"
          >log out</button>
        </div>
      </div>
    </aside>
    <main class="overflow-y-auto px-10 py-8"><slot /></main>
  </div>
</template>
