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
  <div class="shell">
    <aside class="side">
      <div class="brand"><Logo :height="26" /></div>
      <UNavigationMenu
        orientation="vertical"
        :items="items"
        class="nav"
      />
      <div class="spacer" />
      <div class="user">
        <UAvatar :alt="initials" size="sm" />
        <div class="user-text">
          <div class="user-name">{{ auth.user?.username }}</div>
          <button class="logout" @click="logout">log out</button>
        </div>
      </div>
    </aside>
    <main class="main">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.shell {
  display: grid;
  grid-template-columns: 240px 1fr;
  min-height: 100vh;
  background: var(--ui-bg);
}
.side {
  display: flex;
  flex-direction: column;
  background: var(--ui-bg-muted);
  border-right: 1px solid var(--ui-border);
  padding: 1rem 0.75rem;
  gap: 0.75rem;
  overflow: hidden;
}
.brand { padding: 0.5rem 0.5rem 1rem; }
.nav { width: 100%; }
.spacer { flex: 1; }
.user {
  display: flex; gap: 0.6rem; align-items: center;
  padding: 0.75rem 0.5rem 0.25rem; border-top: 1px solid var(--ui-border);
}
.user-text { display: flex; flex-direction: column; line-height: 1.2; }
.user-name { font-size: 0.9rem; color: var(--ui-text-highlighted); }
.logout {
  background: none; border: 0; color: var(--ui-text-muted);
  font-size: 0.75rem; text-align: left; padding: 0; cursor: pointer;
}
.logout:hover { color: var(--ui-primary); }
.main { padding: 2rem 2.5rem; overflow-y: auto; }
</style>
