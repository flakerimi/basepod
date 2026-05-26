<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Logo from '@/components/Logo.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const username = ref('admin')
const password = ref('')
const confirmPassword = ref('')
const submitting = ref(false)
const loadingStatus = ref(true)
const localError = ref<string | null>(null)

const setupMode = computed(() => auth.setupComplete === false)
const title = computed(() => setupMode.value ? 'Create your BasePod admin user' : 'Sign in to your BasePod server')
const passwordAutocomplete = computed(() => setupMode.value ? 'new-password' : 'current-password')

onMounted(async () => {
  try {
    await auth.loadStatus()
  } finally {
    loadingStatus.value = false
  }
})

async function submit() {
  localError.value = null
  if (setupMode.value) {
    if (password.value.length < 8) {
      localError.value = 'password must be at least 8 characters'
      return
    }
    if (password.value !== confirmPassword.value) {
      localError.value = 'passwords do not match'
      return
    }
  }
  submitting.value = true
  try {
    if (setupMode.value) {
      await auth.setup(username.value, password.value)
    } else {
      await auth.login(username.value, password.value)
    }
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
      <p class="mb-6 mt-2 text-center text-(--ui-text-muted)">{{ title }}</p>
      <form class="flex flex-col gap-3.5" @submit.prevent="submit">
        <UFormField label="Username">
          <UInput v-model="username" autocomplete="username" />
        </UFormField>
        <UFormField label="Password">
          <UInput v-model="password" type="password" :autocomplete="passwordAutocomplete" />
        </UFormField>
        <UFormField v-if="setupMode" label="Confirm password">
          <UInput v-model="confirmPassword" type="password" autocomplete="new-password" />
        </UFormField>
        <UAlert v-if="localError || auth.error" color="error" :title="localError || auth.error || undefined" />
        <UButton type="submit" block :loading="submitting || loadingStatus">
          {{ setupMode ? 'Create admin user' : 'Sign in' }}
        </UButton>
      </form>
    </div>
  </div>
</template>
