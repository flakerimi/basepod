<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Logo from '@/components/Logo.vue'

const router = useRouter()
const auth = useAuthStore()

const navItems = [
  { to: '/', label: 'Dashboard', icon: 'i-lucide-home' },
  { to: '/apps', label: 'Apps', icon: 'i-lucide-boxes' },
  { to: '/templates', label: 'Templates', icon: 'i-lucide-shapes' },
  { to: '/settings', label: 'Settings', icon: 'i-lucide-settings' },
]

const initials = computed(() => (auth.user?.username ?? '?').slice(0, 2).toUpperCase())

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="shell">
    <aside class="side">
      <div class="brand"><Logo height="28" /></div>
      <nav class="nav">
        <RouterLink v-for="n in navItems" :key="n.to" :to="n.to" class="nav-item" active-class="nav-item-active">
          <UIcon :name="n.icon" />
          <span>{{ n.label }}</span>
        </RouterLink>
      </nav>
      <div class="user">
        <UAvatar :alt="initials" />
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
}
.side {
  display: flex;
  flex-direction: column;
  background: var(--ui-bg-elevated);
  border-right: 1px solid var(--ui-border);
  padding: 1.25rem 1rem;
  gap: 1rem;
}
.brand {
  padding: 0.5rem 0.25rem 1.5rem;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  flex: 1;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.7rem;
  border-radius: 0.5rem;
  color: var(--ui-text-muted);
  text-decoration: none;
  font-size: 0.92rem;
}
.nav-item:hover { background: var(--ui-bg); color: var(--ui-text); }
.nav-item-active { background: var(--color-primary-500); color: white !important; }
.user {
  display: flex; gap: 0.6rem; align-items: center;
  padding: 0.6rem 0.4rem; border-top: 1px solid var(--ui-border);
}
.user-text { display: flex; flex-direction: column; }
.user-name { font-size: 0.9rem; }
.logout { background: none; border: 0; color: var(--ui-text-muted); font-size: 0.75rem; text-align: left; padding: 0; cursor: pointer; }
.logout:hover { color: var(--color-primary-500); }
.main { padding: 2rem; overflow-y: auto; }
</style>
