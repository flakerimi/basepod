<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAppsStore, type App } from '@/stores/apps'
import { api, sse } from '@/api/client'

const props = defineProps<{ name: string }>()
const apps = useAppsStore()
const app = ref<App | null>(null)
const tab = ref<'overview' | 'env' | 'domains' | 'logs' | 'versions'>('overview')
const logs = ref<string[]>([])
const env = ref<Record<string, string>>({})
const newDomain = ref('')
const newKey = ref('')
const newVal = ref('')

let stopLogs: (() => void) | undefined

onMounted(load)

async function load() {
  app.value = await apps.get(props.name)
  const e = await api.get<{ env: Record<string, string> }>(`/api/v1/apps/${props.name}/env`)
  env.value = e.env ?? {}
}

function startLogs() {
  if (stopLogs) return
  logs.value = []
  stopLogs = sse(`/api/v1/apps/${props.name}/logs`, (e) => {
    logs.value.push(e.data)
    if (logs.value.length > 500) logs.value.shift()
  })
}

function stopLogStream() {
  stopLogs?.()
  stopLogs = undefined
}

async function saveEnv() {
  await api.put(`/api/v1/apps/${props.name}/env`, { env: env.value })
}

async function addDomain() {
  if (!newDomain.value) return
  await api.post(`/api/v1/apps/${props.name}/domains`, { domain: newDomain.value })
  newDomain.value = ''
  await load()
}

async function removeDomain(d: string) {
  await api.del(`/api/v1/apps/${props.name}/domains/${encodeURIComponent(d)}`)
  await load()
}

async function restart() {
  await api.post(`/api/v1/apps/${props.name}/restart`)
}
</script>

<template>
  <div v-if="app" class="page">
    <header class="head">
      <div>
        <h1>{{ app.name }}</h1>
        <p class="muted">{{ app.image_repo || '—' }} · {{ app.current_version || 'not deployed' }}</p>
      </div>
      <div class="actions">
        <UButton color="neutral" variant="outline" icon="i-lucide-rotate-cw" @click="restart">Restart</UButton>
      </div>
    </header>

    <UTabs
      v-model="tab"
      :items="[
        { label: 'Overview', value: 'overview' },
        { label: 'Env', value: 'env' },
        { label: 'Domains', value: 'domains' },
        { label: 'Logs', value: 'logs' },
        { label: 'Versions', value: 'versions' },
      ]"
    />

    <section v-if="tab === 'overview'" class="section">
      <dl class="kv">
        <dt>Instances</dt><dd>{{ app.instances }}</dd>
        <dt>Strategy</dt><dd>{{ app.deploy_strategy }}</dd>
        <dt>Internal only</dt><dd>{{ app.internal_only ? 'yes' : 'no' }}</dd>
        <dt>Ports</dt><dd>{{ app.ports.join(', ') || '—' }}</dd>
        <dt>Volumes</dt>
        <dd>
          <ul>
            <li v-for="v in app.volumes" :key="v.container">
              {{ v.container }} ← {{ v.host || v.named_volume }}
            </li>
          </ul>
        </dd>
      </dl>
    </section>

    <section v-else-if="tab === 'env'" class="section">
      <div v-for="(value, key) in env" :key="String(key)" class="row">
        <UInput :model-value="String(key)" disabled />
        <UInput v-model="env[String(key)]" type="password" />
        <UButton color="error" variant="ghost" icon="i-lucide-x" @click="delete env[String(key)]" />
      </div>
      <div class="row">
        <UInput placeholder="NEW_KEY" v-model="newKey" />
        <UInput placeholder="value" v-model="newVal" />
        <UButton icon="i-lucide-plus" @click="env[newKey] = newVal; newKey = ''; newVal = ''" :disabled="!newKey" />
      </div>
      <div style="margin-top: 1rem"><UButton @click="saveEnv">Save env</UButton></div>
    </section>

    <section v-else-if="tab === 'domains'" class="section">
      <ul class="domains">
        <li v-for="d in app.domains" :key="d.domain">
          {{ d.domain }}
          <span class="muted">— {{ d.tls_state }}</span>
          <UButton size="xs" variant="ghost" color="error" @click="removeDomain(d.domain)">Remove</UButton>
        </li>
      </ul>
      <div class="row">
        <UInput v-model="newDomain" placeholder="example.com" />
        <UButton @click="addDomain">Attach</UButton>
      </div>
    </section>

    <section v-else-if="tab === 'logs'" class="section">
      <div class="logs-bar">
        <UButton v-if="!stopLogs" icon="i-lucide-play" @click="startLogs">Stream</UButton>
        <UButton v-else color="neutral" icon="i-lucide-square" @click="stopLogStream">Stop</UButton>
      </div>
      <pre class="logs">{{ logs.join('\n') || '— start streaming to see container logs —' }}</pre>
    </section>

    <section v-else-if="tab === 'versions'" class="section">
      <p class="muted">Coming soon: version history + rollback.</p>
    </section>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 1.5rem; }
.head { display: flex; justify-content: space-between; align-items: start; }
.head h1 { margin: 0; }
.muted { color: var(--ui-text-muted); }
.section { background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); border-radius: 1rem; padding: 1.25rem; }
.kv { display: grid; grid-template-columns: 140px 1fr; gap: 0.6rem 1.2rem; margin: 0; }
.kv dt { color: var(--ui-text-muted); }
.row { display: grid; grid-template-columns: 1fr 1fr auto; gap: 0.5rem; margin-bottom: 0.5rem; }
.domains { list-style: none; padding: 0; margin: 0 0 1rem; }
.domains li { display: flex; gap: 0.6rem; align-items: center; padding: 0.4rem 0; border-bottom: 1px solid var(--ui-border); }
.logs-bar { margin-bottom: 0.75rem; }
.logs {
  background: #000;
  color: #d8e6ed;
  padding: 1rem;
  border-radius: 0.5rem;
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  max-height: 60vh;
  overflow: auto;
  font-size: 0.85rem;
  margin: 0;
}
</style>
