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
  <div class="grid min-h-screen place-items-center bg-(--ui-bg)">
    <div class="w-[360px] rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-8">
      <div class="flex justify-center"><Logo :height="40" /></div>
      <p class="mb-6 mt-2 text-center text-(--ui-text-muted)">Sign in to your BasePod server</p>
      <form class="flex flex-col gap-3.5" @submit.prevent="submit">
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
