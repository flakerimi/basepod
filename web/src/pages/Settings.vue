<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const root = ref('')
const email = ref('')
const provider = ref('')
const token = ref('')
const saving = ref(false)

onMounted(async () => {
  const r = await api.get<{ settings: Record<string, string> }>('/api/v1/settings')
  root.value = r.settings.root_domain ?? ''
  email.value = r.settings.acme_email ?? ''
  provider.value = r.settings.dns_provider ?? ''
})

async function save() {
  saving.value = true
  try {
    const payload: Record<string, string> = {
      root_domain: root.value,
      acme_email: email.value,
      dns_provider: provider.value,
    }
    if (token.value) payload.dns_token = token.value
    await api.put('/api/v1/settings', payload)
    token.value = ''
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="page">
    <h1>Settings</h1>
    <section class="card">
      <h2>Domains & TLS</h2>
      <UFormField label="Root domain" help="Apps get <app>.<root> automatically.">
        <UInput v-model="root" placeholder="example.com" />
      </UFormField>
      <UFormField label="ACME email" help="Used by Let's Encrypt to notify about cert problems.">
        <UInput v-model="email" type="email" />
      </UFormField>
      <UFormField label="DNS provider (optional, enables wildcard cert)">
        <UInput v-model="provider" placeholder="cloudflare" />
      </UFormField>
      <UFormField label="DNS provider API token">
        <UInput v-model="token" type="password" placeholder="leave empty to keep existing" />
      </UFormField>
      <UButton @click="save" :loading="saving">Save</UButton>
    </section>
  </div>
</template>

<style scoped>
.page h1 { margin-top: 0; }
.card { background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); border-radius: 1rem; padding: 1.5rem; max-width: 540px; }
.card h2 { margin-top: 0; }
section { display: flex; flex-direction: column; gap: 0.75rem; }
</style>
