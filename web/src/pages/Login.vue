<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Logo from '@/components/Logo.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const username = ref('admin')
const password = ref('')
const submitting = ref(false)

async function submit() {
  submitting.value = true
  try {
    await auth.login(username.value, password.value)
    const next = (route.query.next as string) || '/'
    router.replace(next)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="wrap">
    <div class="card">
      <div class="logo"><Logo height="44" /></div>
      <p class="sub">Sign in to your BasePod server</p>
      <form @submit.prevent="submit" class="form">
        <UFormField label="Username">
          <UInput v-model="username" autocomplete="username" />
        </UFormField>
        <UFormField label="Password">
          <UInput v-model="password" type="password" autocomplete="current-password" />
        </UFormField>
        <UAlert v-if="auth.error" color="error" :title="auth.error" />
        <UButton type="submit" block :loading="submitting">Sign in</UButton>
      </form>
    </div>
  </div>
</template>

<style scoped>
.wrap { min-height: 100vh; display: grid; place-items: center; background: var(--ui-bg); }
.card { width: 360px; padding: 2rem; border-radius: 1rem; background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); }
.logo { display: flex; justify-content: center; }
.sub { text-align: center; color: var(--ui-text-muted); margin: 0.5rem 0 1.5rem; }
.form { display: flex; flex-direction: column; gap: 0.85rem; }
</style>
