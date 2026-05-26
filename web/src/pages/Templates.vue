<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { api } from '@/api/client'

interface Field { key: string; label: string; type?: string; required?: boolean; default?: string }
interface Template {
  id: string
  name: string
  version?: string
  description?: string
  fields?: Field[]
}

const templates = ref<Template[]>([])
const loading = ref(false)
const open = ref(false)
const selected = ref<Template | null>(null)
const appName = ref('')
const fields = ref<Record<string, string>>({})
const installing = ref(false)

onMounted(load)

async function load() {
  loading.value = true
  try {
    const r = await api.get<{ templates: Template[] }>('/api/v1/templates')
    templates.value = r.templates ?? []
  } finally {
    loading.value = false
  }
}

function start(t: Template) {
  selected.value = t
  appName.value = t.id
  fields.value = {}
  for (const f of t.fields ?? []) {
    fields.value[f.key] = f.default ?? ''
  }
  open.value = true
}

const formFields = computed<Field[]>(() => selected.value?.fields ?? [])

async function install() {
  if (!selected.value) return
  installing.value = true
  try {
    await api.post('/api/v1/templates/install', {
      template_id: selected.value.id,
      app_name: appName.value,
      fields: fields.value,
    })
    open.value = false
  } finally {
    installing.value = false
  }
}
</script>

<template>
  <div>
    <h1 class="m-0 mb-1 text-2xl font-semibold">One-click apps</h1>
    <p class="mb-6 mt-0 text-(--ui-text-muted)">Spin up databases, queues, and other services with a single click.</p>

    <div v-if="loading">Loading…</div>

    <div v-else class="grid gap-4" style="grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));">
      <div
        v-for="t in templates"
        :key="t.id"
        class="flex cursor-pointer flex-col gap-2 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5 hover:border-(--ui-primary)"
        @click="start(t)"
      >
        <div class="flex items-center gap-2 font-semibold">
          {{ t.name }}
          <span v-if="t.version" class="text-sm font-normal text-(--ui-text-muted)">{{ t.version }}</span>
        </div>
        <p class="flex-1 text-sm text-(--ui-text-muted)">{{ t.description }}</p>
        <UButton size="sm" variant="soft">Install</UButton>
      </div>
    </div>

    <UModal v-model:open="open" :title="selected?.name ?? 'Install'">
      <template #body>
        <UFormField label="App name">
          <UInput v-model="appName" />
        </UFormField>
        <UFormField v-for="f in formFields" :key="f.key" :label="f.label" :required="f.required">
          <UInput v-model="fields[f.key]" :type="f.type === 'password' ? 'password' : 'text'" />
        </UFormField>
      </template>
      <template #footer>
        <UButton color="neutral" variant="ghost" @click="open = false">Cancel</UButton>
        <UButton :loading="installing" @click="install">Install</UButton>
      </template>
    </UModal>
  </div>
</template>
