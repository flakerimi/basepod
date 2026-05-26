<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const root = ref('')
const email = ref('')
const provider = ref('')
const token = ref('')
const adminSub = ref('bp')
const saving = ref(false)

onMounted(async () => {
  const r = await api.get<{ settings: Record<string, string> }>('/api/v1/settings')
  root.value = r.settings.root_domain ?? ''
  email.value = r.settings.acme_email ?? ''
  provider.value = r.settings.dns_provider ?? ''
  adminSub.value = r.settings.admin_subdomain ?? 'bp'
})

async function save() {
  saving.value = true
  try {
    const payload: Record<string, string> = {
      root_domain: root.value,
      acme_email: email.value,
      dns_provider: provider.value,
      admin_subdomain: adminSub.value,
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
  <div>
    <h1 class="m-0 mb-6 text-2xl font-semibold">Settings</h1>
    <section class="flex max-w-[540px] flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-6">
      <h2 class="mt-0 text-lg font-semibold">Domains &amp; TLS</h2>
      <UFormField label="Root domain" help="Apps get <app>.<root> automatically.">
        <UInput v-model="root" placeholder="example.com" />
      </UFormField>
      <UFormField label="Admin subdomain" help="Reserved for this dashboard, e.g. bp.<root>. Cannot be used as an app name.">
        <UInput v-model="adminSub" placeholder="bp" />
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
      <UButton :loading="saving" @click="save">Save</UButton>
    </section>
  </div>
</template>
