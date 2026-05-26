<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useAppsStore, type App } from '@/stores/apps'
import { api, sse } from '@/api/client'

const props = defineProps<{ name: string }>()
const apps = useAppsStore()
const app = ref<App | null>(null)
type Tab = 'overview' | 'env' | 'domains' | 'logs' | 'versions' | 'git'
const tab = ref<Tab>('overview')
const logs = ref<string[]>([])
const env = ref<Record<string, string>>({})
const newDomain = ref('')
const newKey = ref('')
const newVal = ref('')
const envMode = ref<'rows' | 'bulk'>('rows')
const envBulk = ref('')
const envRevealed = ref<Record<string, boolean>>({})
const envRestartOnSave = ref(true)
const envSaving = ref(false)
const envSavedMsg = ref('')

interface AppConfig {
  deploy_strategy: string
  instances: number
  memory_mb: number
  cpu_pct: number
  healthcheck_path: string
  internal_only: boolean
}
const cfg = ref<AppConfig>({
  deploy_strategy: 'blue_green',
  instances: 1,
  memory_mb: 0,
  cpu_pct: 0,
  healthcheck_path: '',
  internal_only: false,
})
const cfgBase = ref<AppConfig>({ ...cfg.value })
const cfgSaving = ref(false)
const cfgSavedMsg = ref('')
const cfgDirty = computed(() => JSON.stringify(cfg.value) !== JSON.stringify(cfgBase.value))

interface AppVersion {
  id: string
  version: string
  image_tag: string
  status: string
  deployed_at: number
  log_excerpt?: string
}
const versions = ref<AppVersion[]>([])
const rollingBack = ref<string | null>(null)

const newPort = ref<number>(0)
const newVolContainer = ref('')
const newVolHost = ref('')

// Git tab state
interface GitConfig {
  url: string
  branch: string
  dockerfile: string
  has_credential: boolean
  has_webhook: boolean
  webhook_url: string
}

function blankGitConfig(): GitConfig {
  return { url: '', branch: 'main', dockerfile: 'Dockerfile', has_credential: false, has_webhook: false, webhook_url: '' }
}

const git = ref<GitConfig>(blankGitConfig())
const savedGit = ref<GitConfig>(blankGitConfig())
const gitToken = ref('')
const gitUser = ref('')
const gitPass = ref('')
const gitAuthMode = ref<'token' | 'userpass'>('token')
const gitSaving = ref(false)
const gitDeploying = ref(false)
const gitWebhookRevealed = ref<string | null>(null)
const gitWebhookURL = ref('')
const gitDirty = computed(() => {
  const current = {
    url: git.value.url,
    branch: git.value.branch || 'main',
    dockerfile: git.value.dockerfile || 'Dockerfile',
  }
  const saved = {
    url: savedGit.value.url,
    branch: savedGit.value.branch || 'main',
    dockerfile: savedGit.value.dockerfile || 'Dockerfile',
  }
  return JSON.stringify(current) !== JSON.stringify(saved)
    || Boolean(gitToken.value || gitPass.value || (gitAuthMode.value === 'userpass' && gitUser.value))
})

let stopLogs: (() => void) | undefined

onMounted(load)

async function load() {
  app.value = await apps.get(props.name)
  cfg.value = {
    deploy_strategy: app.value.deploy_strategy,
    instances: app.value.instances,
    memory_mb: app.value.memory_mb,
    cpu_pct: app.value.cpu_pct,
    healthcheck_path: (app.value as any).healthcheck_path ?? '',
    internal_only: app.value.internal_only,
  }
  cfgBase.value = { ...cfg.value }
  const e = await api.get<{ env: Record<string, string> }>(`/api/v1/apps/${props.name}/env`)
  env.value = e.env ?? {}
  await loadGit()
  await loadVersions()
}

async function loadVersions() {
  try {
    const r = await api.get<{ versions: AppVersion[] }>(`/api/v1/apps/${props.name}/versions`)
    versions.value = r.versions ?? []
  } catch { /* ignore */ }
}

async function rollback(version: string) {
  if (!confirm(`Roll back to ${version}?`)) return
  rollingBack.value = version
  try {
    await api.post(`/api/v1/apps/${props.name}/rollback`, { version })
    await load()
  } finally {
    rollingBack.value = null
  }
}

function formatTs(ts: number) {
  if (!ts) return '—'
  return new Date(ts * 1000).toLocaleString()
}

async function addPort() {
  if (!newPort.value || newPort.value < 1) return
  await api.post(`/api/v1/apps/${props.name}/ports`, { port: newPort.value })
  newPort.value = 0
  app.value = await apps.get(props.name)
}

async function removePort(p: number) {
  await api.del(`/api/v1/apps/${props.name}/ports/${p}`)
  app.value = await apps.get(props.name)
}

async function addVolume() {
  if (!newVolContainer.value || !newVolHost.value) return
  await api.post(`/api/v1/apps/${props.name}/volumes`, {
    container: newVolContainer.value,
    host: newVolHost.value,
  })
  newVolContainer.value = ''
  newVolHost.value = ''
  app.value = await apps.get(props.name)
}

async function removeVolume(containerPath: string) {
  await api.del(`/api/v1/apps/${props.name}/volumes/${encodeURIComponent(containerPath)}`)
  app.value = await apps.get(props.name)
}

async function saveCfg() {
  cfgSaving.value = true
  cfgSavedMsg.value = ''
  try {
    await api.patch(`/api/v1/apps/${props.name}`, cfg.value)
    cfgBase.value = { ...cfg.value }
    cfgSavedMsg.value = 'saved'
    setTimeout(() => (cfgSavedMsg.value = ''), 3000)
    app.value = await apps.get(props.name)
  } catch (err: any) {
    cfgSavedMsg.value = 'error: ' + (err?.error ?? 'save failed')
  } finally {
    cfgSaving.value = false
  }
}

function resetCfg() {
  cfg.value = { ...cfgBase.value }
}

async function loadGit() {
  try {
    const g = await api.get<GitConfig>(`/api/v1/apps/${props.name}/git`)
    const normalized = {
      ...blankGitConfig(),
      ...g,
      branch: g.branch || 'main',
      dockerfile: g.dockerfile || 'Dockerfile',
    }
    git.value = { ...normalized }
    savedGit.value = { ...normalized }
    gitWebhookURL.value = normalized.webhook_url
  } catch {
    const empty = blankGitConfig()
    git.value = { ...empty }
    savedGit.value = { ...empty }
    gitWebhookURL.value = ''
  }
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

function isSecretKey(k: string) {
  return /KEY|TOKEN|PASS|SECRET|DSN|AUTH|PRIVATE/i.test(k)
}

function bulkText(): string {
  return Object.entries(env.value)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
    .join('\n')
}

function syncBulkFromRows() {
  envBulk.value = bulkText()
}

function parseBulk(text: string): { env: Record<string, string>; error?: string } {
  const out: Record<string, string> = {}
  const lines = text.split(/\r?\n/)
  for (const raw of lines) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    const eq = line.indexOf('=')
    if (eq <= 0) return { env: {}, error: `bad line: ${line}` }
    const key = line.slice(0, eq).trim()
    const val = line.slice(eq + 1).replace(/^["']|["']$/g, '')
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(key)) return { env: {}, error: `invalid key: ${key}` }
    out[key] = val
  }
  return { env: out }
}

function addRow() {
  if (!newKey.value) return
  env.value = { ...env.value, [newKey.value]: newVal.value }
  newKey.value = ''
  newVal.value = ''
}

function removeKey(k: string) {
  const next = { ...env.value }
  delete next[k]
  env.value = next
  delete envRevealed.value[k]
}

function toggleReveal(k: string) {
  envRevealed.value = { ...envRevealed.value, [k]: !envRevealed.value[k] }
}

async function saveEnv() {
  envSaving.value = true
  envSavedMsg.value = ''
  try {
    let payload: Record<string, string> = env.value
    if (envMode.value === 'bulk') {
      const r = parseBulk(envBulk.value)
      if (r.error) {
        envSavedMsg.value = 'error: ' + r.error
        return
      }
      payload = r.env
      env.value = payload
    }
    const qs = envRestartOnSave.value ? '?restart=1' : ''
    const resp = await api.put<{ ok: boolean; restarted: boolean }>(
      `/api/v1/apps/${props.name}/env${qs}`,
      { env: payload },
    )
    envSavedMsg.value = resp.restarted ? 'saved + restarted' : 'saved'
    setTimeout(() => (envSavedMsg.value = ''), 3000)
  } catch (err: any) {
    envSavedMsg.value = 'error: ' + (err?.error ?? 'save failed')
  } finally {
    envSaving.value = false
  }
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

async function saveGit() {
  gitSaving.value = true
  try {
    const body: Record<string, string> = {
      url: git.value.url,
      branch: git.value.branch || 'main',
      dockerfile: git.value.dockerfile || 'Dockerfile',
    }
    if (gitAuthMode.value === 'token' && gitToken.value) {
      body.token = gitToken.value
    } else if (gitAuthMode.value === 'userpass' && gitPass.value) {
      body.username = gitUser.value
      body.password = gitPass.value
    }
    await api.put(`/api/v1/apps/${props.name}/git`, body)
    gitToken.value = ''
    gitUser.value = ''
    gitPass.value = ''
    await loadGit()
  } finally {
    gitSaving.value = false
  }
}

async function rotateWebhook() {
  const r = await api.post<{ secret: string; webhook_url: string }>(`/api/v1/apps/${props.name}/webhook-secret`)
  gitWebhookRevealed.value = r.secret
  gitWebhookURL.value = r.webhook_url
  await loadGit()
}

async function deployFromGit() {
  gitDeploying.value = true
  try {
    await api.post(`/api/v1/apps/${props.name}/deploy`, { from_stored: true })
  } finally {
    gitDeploying.value = false
  }
}

function copy(text: string) {
  navigator.clipboard?.writeText(text)
}
</script>

<template>
  <div v-if="app" class="flex flex-col gap-6">
    <header class="flex items-start justify-between">
      <div>
        <h1 class="m-0 text-2xl font-semibold">{{ app.name }}</h1>
        <p class="m-0 text-(--ui-text-muted)">{{ app.image_repo || '—' }} · {{ app.current_version || 'not deployed' }}</p>
      </div>
      <div>
        <UButton color="neutral" variant="outline" icon="i-lucide-rotate-cw" @click="restart">Restart</UButton>
      </div>
    </header>

    <UTabs
      v-model="tab"
      :items="[
        { label: 'Overview', value: 'overview' },
        { label: 'Env', value: 'env' },
        { label: 'Domains', value: 'domains' },
        { label: 'Git', value: 'git', icon: 'i-lucide-git-branch' },
        { label: 'Logs', value: 'logs' },
        { label: 'Versions', value: 'versions' },
      ]"
    />

    <section v-if="tab === 'overview'" class="flex flex-col gap-4">
      <!-- Status banner -->
      <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) px-5 py-4">
        <div class="flex items-center gap-3">
          <span
            class="size-2.5 rounded-full ring-4"
            :class="app.current_version
              ? 'bg-green-500 ring-green-500/25'
              : 'bg-(--ui-border) ring-(--ui-border)/30'"
          />
          <div>
            <div class="font-semibold">{{ app.current_version ? 'Deployed' : 'Not deployed' }}</div>
            <div class="text-sm text-(--ui-text-muted)">
              <code v-if="app.image_repo" class="bg-transparent p-0">{{ app.image_repo }}</code>
              <span v-else>no image yet</span>
              <span v-if="app.current_version"> · <code class="bg-transparent p-0">{{ app.current_version }}</code></span>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <span class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 font-mono text-xs text-(--ui-text-muted)">{{ app.deploy_strategy }}</span>
          <span v-if="app.instances > 1" class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 font-mono text-xs text-(--ui-text-muted)">×{{ app.instances }}</span>
          <span v-if="app.internal_only" class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 font-mono text-xs text-(--ui-text-muted)">internal</span>
          <span v-if="app.memory_mb" class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 font-mono text-xs text-(--ui-text-muted)">{{ app.memory_mb }} MB</span>
          <span v-if="app.cpu_pct" class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 font-mono text-xs text-(--ui-text-muted)">{{ app.cpu_pct }}% CPU</span>
        </div>
      </div>

      <!-- Two-column grid -->
      <div class="grid items-start gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <!-- LEFT: configuration form -->
        <div class="flex flex-col gap-4 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
          <div class="flex items-baseline justify-between gap-3">
            <h3 class="m-0 text-base font-semibold">Configuration</h3>
            <span v-if="cfgSavedMsg" class="text-xs text-(--ui-text-muted)">{{ cfgSavedMsg }}</span>
          </div>

          <div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
            <UFormField label="Deploy strategy">
              <USelect
                v-model="cfg.deploy_strategy"
                :items="[{ label: 'blue/green', value: 'blue_green' }, { label: 'stop/start', value: 'stop_start' }]"
              />
            </UFormField>
            <UFormField label="Instances">
              <UInput v-model.number="cfg.instances" type="number" min="1" />
            </UFormField>
            <UFormField label="Memory (MB)" help="0 = unlimited">
              <UInput v-model.number="cfg.memory_mb" type="number" min="0" />
            </UFormField>
            <UFormField label="CPU (%)" help="0 = unlimited">
              <UInput v-model.number="cfg.cpu_pct" type="number" min="0" max="800" />
            </UFormField>
            <UFormField label="Healthcheck path" class="md:col-span-2">
              <UInput v-model="cfg.healthcheck_path" placeholder="/healthz" />
            </UFormField>
            <div class="flex items-center justify-between gap-3 rounded-lg bg-(--ui-bg-muted) px-3 py-2 md:col-span-2">
              <div>
                <div class="text-sm font-medium">Internal only</div>
                <div class="text-xs text-(--ui-text-muted)">Excluded from public Caddy routing</div>
              </div>
              <USwitch v-model="cfg.internal_only" />
            </div>
          </div>

          <div class="flex items-center gap-2 border-t border-(--ui-border) pt-3">
            <UButton :loading="cfgSaving" :disabled="!cfgDirty" @click="saveCfg">Save changes</UButton>
            <UButton v-if="cfgDirty" color="neutral" variant="ghost" @click="resetCfg">Reset</UButton>
          </div>
        </div>

        <!-- RIGHT: ports + volumes stacked -->
        <div class="flex flex-col gap-4">
          <!-- Ports -->
          <div class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
            <div class="flex items-baseline justify-between gap-3">
              <h3 class="m-0 text-base font-semibold">Ports</h3>
              <span class="text-xs text-(--ui-text-muted)">applied on redeploy</span>
            </div>
            <ul v-if="app.ports?.length" class="m-0 flex flex-wrap gap-1.5 p-0">
              <li
                v-for="p in (app.ports ?? [])"
                :key="p"
                class="inline-flex items-center gap-1.5 rounded-lg bg-(--ui-bg-muted) px-2.5 py-1.5 font-mono text-sm"
              >
                <code class="bg-transparent p-0">{{ p }}</code>
                <button
                  class="size-4 cursor-pointer text-(--ui-text-muted) hover:text-red-600"
                  aria-label="remove"
                  @click="removePort(p)"
                >×</button>
              </li>
            </ul>
            <p v-else class="m-0 text-sm text-(--ui-text-muted)">No ports defined.</p>
            <div class="flex items-stretch gap-2">
              <UInput v-model.number="newPort" type="number" min="1" max="65535" placeholder="3000" size="sm" class="flex-1" />
              <UButton size="sm" icon="i-lucide-plus" :disabled="!newPort" @click="addPort">Add</UButton>
            </div>
          </div>

          <!-- Volumes -->
          <div class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
            <div class="flex items-baseline justify-between gap-3">
              <h3 class="m-0 text-base font-semibold">Volumes</h3>
              <span class="text-xs text-(--ui-text-muted)"><code class="bg-transparent p-0">~/</code> expands on save</span>
            </div>
            <ul v-if="app.volumes?.length" class="m-0 flex flex-col gap-1.5 p-0">
              <li
                v-for="v in (app.volumes ?? [])"
                :key="v.container"
                class="flex items-center gap-3 rounded-lg bg-(--ui-bg-muted) px-3 py-2 text-sm"
              >
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <code class="bg-transparent p-0 font-mono text-xs">{{ v.container }}</code>
                  <span class="text-xs text-(--ui-text-muted)">←</span>
                  <code class="truncate bg-transparent p-0 font-mono text-xs text-(--ui-text-muted)">{{ v.host || v.named_volume }}</code>
                </div>
                <button
                  class="size-4 cursor-pointer text-(--ui-text-muted) hover:text-red-600"
                  aria-label="remove"
                  @click="removeVolume(v.container)"
                >×</button>
              </li>
            </ul>
            <p v-else class="m-0 text-sm text-(--ui-text-muted)">No volumes mounted.</p>
            <div class="grid grid-cols-[1fr_1.6fr_auto] gap-2">
              <UInput v-model="newVolContainer" placeholder="/data" size="sm" />
              <UInput v-model="newVolHost" placeholder="~/BasePodData/myapp/data" size="sm" />
              <UButton size="sm" icon="i-lucide-plus" :disabled="!newVolContainer || !newVolHost" @click="addVolume">Add</UButton>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section v-else-if="tab === 'env'" class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
      <div class="flex items-center justify-between">
        <h3 class="m-0 text-base font-semibold">Environment variables</h3>
        <URadioGroup
          v-model="envMode"
          :items="[{ label: 'Rows', value: 'rows' }, { label: 'Bulk (.env)', value: 'bulk' }]"
          orientation="horizontal"
          size="sm"
          @update:model-value="(v: string) => v === 'bulk' && syncBulkFromRows()"
        />
      </div>

      <template v-if="envMode === 'rows'">
        <div v-for="(value, key) in env" :key="String(key)" class="grid grid-cols-[1fr_2fr_auto_auto] items-center gap-2">
          <UInput :model-value="String(key)" disabled class="font-mono" />
          <UInput
            v-model="env[String(key)]"
            :type="envRevealed[String(key)] || !isSecretKey(String(key)) ? 'text' : 'password'"
          />
          <UButton
            v-if="isSecretKey(String(key))"
            size="xs"
            variant="ghost"
            :icon="envRevealed[String(key)] ? 'i-lucide-eye-off' : 'i-lucide-eye'"
            @click="toggleReveal(String(key))"
          />
          <UButton size="xs" color="error" variant="ghost" icon="i-lucide-x" @click="removeKey(String(key))" />
        </div>
        <div class="grid grid-cols-[1fr_2fr_auto_auto] items-center gap-2">
          <UInput v-model="newKey" placeholder="NEW_KEY" class="font-mono" />
          <UInput v-model="newVal" placeholder="value" />
          <span />
          <UButton size="xs" icon="i-lucide-plus" :disabled="!newKey" @click="addRow" />
        </div>
      </template>

      <UTextarea
        v-else
        v-model="envBulk"
        :rows="14"
        autoresize
        :ui="{ base: 'font-mono text-sm' }"
        placeholder="# Paste like a .env file
NODE_ENV=production
DATABASE_URL=postgres://..."
      />

      <div class="flex items-center gap-3 border-t border-(--ui-border) pt-3">
        <UCheckbox v-model="envRestartOnSave" label="Restart container after save" />
        <div class="flex-1" />
        <span v-if="envSavedMsg" class="text-xs text-(--ui-text-muted)">{{ envSavedMsg }}</span>
        <UButton :loading="envSaving" @click="saveEnv">Save env</UButton>
      </div>
    </section>

    <section v-else-if="tab === 'domains'" class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
      <ul v-if="app.domains?.length" class="m-0 flex flex-col gap-0 p-0">
        <li
          v-for="d in (app.domains ?? [])"
          :key="d.domain"
          class="flex items-center gap-2 border-b border-(--ui-border) py-2 last:border-0"
        >
          <span class="flex-1">{{ d.domain }}</span>
          <span class="text-sm text-(--ui-text-muted)">{{ d.tls_state }}</span>
          <UButton size="xs" variant="ghost" color="error" @click="removeDomain(d.domain)">Remove</UButton>
        </li>
      </ul>
      <p v-else class="m-0 text-sm text-(--ui-text-muted)">No custom domains yet.</p>
      <div class="flex gap-2">
        <UInput v-model="newDomain" placeholder="example.com" class="flex-1" />
        <UButton :disabled="!newDomain" @click="addDomain">Attach</UButton>
      </div>
    </section>

    <section v-else-if="tab === 'git'" class="flex flex-col gap-4">
      <!-- Status banner -->
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) px-5 py-4">
        <div class="flex items-center gap-3">
          <UIcon name="i-lucide-git-branch" class="text-2xl text-(--ui-primary)" />
          <div>
            <div class="font-semibold">
              {{ savedGit.url ? 'Connected' : 'Not connected' }}
            </div>
            <div class="text-sm text-(--ui-text-muted)">
              <code v-if="savedGit.url" class="bg-transparent p-0">{{ savedGit.url }}</code>
              <span v-else>Configure a repository to enable git deploys.</span>
              <span v-if="savedGit.url"> · branch <code class="bg-transparent p-0">{{ savedGit.branch || 'main' }}</code></span>
            </div>
          </div>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <span v-if="gitDirty" class="rounded-full border border-amber-600/40 bg-amber-500/10 px-2.5 py-0.5 text-xs text-amber-700">unsaved changes</span>
          <span v-if="savedGit.has_credential" class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 text-xs text-(--ui-text-muted)">credential saved</span>
          <span v-if="savedGit.has_webhook" class="rounded-full border border-green-600/40 bg-green-500/10 px-2.5 py-0.5 text-xs text-green-700">webhook active</span>
          <span v-else class="rounded-full border border-(--ui-border) bg-(--ui-bg-muted) px-2.5 py-0.5 text-xs text-(--ui-text-muted)">webhook disabled</span>
        </div>
      </div>

      <!-- Two columns -->
      <div class="grid items-start gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <!-- LEFT: repository + auth -->
        <div class="flex flex-col gap-4 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
          <div>
            <h3 class="m-0 text-base font-semibold">Repository</h3>
            <p class="m-0 mt-1 text-xs text-(--ui-text-muted)">GitHub, GitLab, Bitbucket — any HTTPS git URL.</p>
          </div>

          <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
            <UFormField label="Repository URL" class="md:col-span-2">
              <UInput v-model="git.url" placeholder="https://github.com/owner/repo" />
            </UFormField>
            <UFormField label="Branch">
              <UInput v-model="git.branch" placeholder="main" />
            </UFormField>
            <UFormField label="Dockerfile path">
              <UInput v-model="git.dockerfile" placeholder="Dockerfile" />
            </UFormField>
          </div>

          <hr class="m-0 border-0 border-t border-(--ui-border)" />

          <div>
            <h4 class="m-0 text-sm font-semibold">Authentication</h4>
            <p class="m-0 mt-1 text-xs text-(--ui-text-muted)">
              Public repos: leave blank. Private GitHub: PAT. GitLab/Bitbucket: user + password.
            </p>
          </div>
          <URadioGroup
            v-model="gitAuthMode"
            :items="[
              { label: 'GitHub PAT / token', value: 'token' },
              { label: 'Username + password', value: 'userpass' },
            ]"
            orientation="horizontal"
          />
          <div v-if="gitAuthMode === 'token'">
            <UFormField :label="savedGit.has_credential ? 'Replace token (leave blank to keep existing)' : 'Token'">
              <UInput v-model="gitToken" type="password" placeholder="ghp_xxx" />
            </UFormField>
          </div>
          <div v-else class="grid grid-cols-1 gap-3 md:grid-cols-2">
            <UFormField label="Username">
              <UInput v-model="gitUser" placeholder="me / oauth2" />
            </UFormField>
            <UFormField :label="savedGit.has_credential ? 'Replace password' : 'Password / app password'">
              <UInput v-model="gitPass" type="password" />
            </UFormField>
          </div>

          <div class="flex items-center gap-2 border-t border-(--ui-border) pt-3">
            <UButton :loading="gitSaving" :disabled="!git.url" @click="saveGit">Save</UButton>
            <UButton
              color="neutral"
              variant="outline"
              icon="i-lucide-rocket"
              :loading="gitDeploying"
              :disabled="!savedGit.url"
              @click="deployFromGit"
            >Force build</UButton>
          </div>
        </div>

        <!-- RIGHT: webhook -->
        <div class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
          <div class="flex items-baseline justify-between gap-3">
            <h3 class="m-0 text-base font-semibold">Webhook</h3>
            <span class="text-xs text-(--ui-text-muted)">push → deploy</span>
          </div>
          <p class="m-0 text-xs text-(--ui-text-muted)">
            GitHub and GitLab POST here on push. BasePod verifies the HMAC signature against the secret before deploying.
          </p>

          <div v-if="savedGit.webhook_url" class="flex flex-col gap-3 rounded-xl border border-(--ui-border) bg-(--ui-bg) p-3">
            <div class="flex flex-col gap-1">
              <span class="text-xs uppercase tracking-wider text-(--ui-text-muted)">Payload URL</span>
              <div class="flex items-center gap-2">
                <code class="flex-1 break-all rounded-md bg-(--ui-bg-muted) px-2 py-1.5 font-mono text-xs">{{ gitWebhookURL || savedGit.webhook_url }}</code>
                <UButton size="xs" variant="ghost" icon="i-lucide-copy" @click="copy(gitWebhookURL || savedGit.webhook_url)" />
              </div>
            </div>

            <div v-if="gitWebhookRevealed" class="flex flex-col gap-1">
              <span class="text-xs uppercase tracking-wider text-(--ui-primary)">Secret · shown once</span>
              <div class="flex items-center gap-2">
                <code class="flex-1 break-all rounded-md bg-(--ui-primary)/10 px-2 py-1.5 font-mono text-xs text-(--ui-primary)">{{ gitWebhookRevealed }}</code>
                <UButton size="xs" variant="ghost" icon="i-lucide-copy" @click="copy(gitWebhookRevealed)" />
              </div>
            </div>
            <div v-else-if="savedGit.has_webhook" class="flex flex-col gap-1">
              <span class="text-xs uppercase tracking-wider text-(--ui-text-muted)">Secret</span>
              <span class="text-sm text-(--ui-text-muted)">configured — rotate to view a new one</span>
            </div>
          </div>

          <UButton variant="outline" icon="i-lucide-key" @click="rotateWebhook">
            {{ savedGit.has_webhook ? 'Rotate webhook secret' : 'Generate webhook secret' }}
          </UButton>

          <details class="rounded-xl border border-(--ui-border) bg-(--ui-bg) px-3 py-2 text-sm">
            <summary class="cursor-pointer font-medium">Connect GitHub or GitLab</summary>
            <ol class="m-0 mt-3 list-decimal space-y-1 pl-5 text-xs leading-relaxed">
              <li>In GitHub: Settings → Webhooks → Add webhook. In GitLab: Settings → Webhooks → Add new webhook.</li>
              <li>Payload URL: paste the URL above</li>
              <li>Content type: <code class="rounded bg-(--ui-bg-muted) px-1.5 py-0.5">application/json</code></li>
              <li>Secret: paste the secret above (after rotate)</li>
              <li>Events: <strong>Just the push event</strong></li>
              <li>Next push to <code class="rounded bg-(--ui-bg-muted) px-1.5 py-0.5">{{ savedGit.branch || 'main' }}</code> triggers a deploy.</li>
            </ol>
          </details>
        </div>
      </div>
    </section>

    <section v-else-if="tab === 'logs'" class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
      <div>
        <UButton v-if="!stopLogs" icon="i-lucide-play" @click="startLogs">Stream</UButton>
        <UButton v-else color="neutral" icon="i-lucide-square" @click="stopLogStream">Stop</UButton>
      </div>
      <pre class="m-0 max-h-[60vh] overflow-auto rounded-lg bg-black p-4 font-mono text-sm text-[#d8e6ed]">{{ logs.join('\n') || '— start streaming to see container logs —' }}</pre>
    </section>

    <section v-else-if="tab === 'versions'" class="flex flex-col gap-3 rounded-2xl border border-(--ui-border) bg-(--ui-bg-elevated) p-5">
      <h3 class="m-0 text-base font-semibold">Deploy history</h3>
      <p class="m-0 text-xs text-(--ui-text-muted)">Last 5 versions are kept. Rollback re-deploys the selected image tag.</p>
      <table v-if="versions.length" class="w-full border-collapse">
        <thead>
          <tr>
            <th class="border-b border-(--ui-border) px-3 py-2.5 text-left text-xs uppercase tracking-wider text-(--ui-text-muted)">Version</th>
            <th class="border-b border-(--ui-border) px-3 py-2.5 text-left text-xs uppercase tracking-wider text-(--ui-text-muted)">Image</th>
            <th class="border-b border-(--ui-border) px-3 py-2.5 text-left text-xs uppercase tracking-wider text-(--ui-text-muted)">Status</th>
            <th class="border-b border-(--ui-border) px-3 py-2.5 text-left text-xs uppercase tracking-wider text-(--ui-text-muted)">Deployed</th>
            <th class="border-b border-(--ui-border) px-3 py-2.5"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="v in versions"
            :key="v.id"
            :class="[
              v.version === app.current_version ? 'bg-(--ui-primary)/5' : '',
            ]"
          >
            <td class="border-b border-(--ui-border) px-3 py-2.5">
              <code class="text-sm">{{ v.version }}</code>
              <span v-if="v.version === app.current_version" class="ml-2 inline-block rounded bg-(--ui-primary) px-1.5 py-0.5 text-[0.65rem] text-white">current</span>
            </td>
            <td class="border-b border-(--ui-border) px-3 py-2.5"><code class="text-sm">{{ v.image_tag || '—' }}</code></td>
            <td class="border-b border-(--ui-border) px-3 py-2.5">
              <span
                class="text-sm"
                :class="{
                  'text-green-600': v.status === 'succeeded',
                  'text-red-600': v.status === 'failed',
                  'text-(--ui-primary)': ['deploying','building','cloning'].includes(v.status),
                }"
              >{{ v.status }}</span>
            </td>
            <td class="border-b border-(--ui-border) px-3 py-2.5">{{ formatTs(v.deployed_at) }}</td>
            <td class="border-b border-(--ui-border) px-3 py-2.5">
              <UButton
                v-if="v.version !== app.current_version && v.status === 'succeeded'"
                size="xs"
                variant="outline"
                icon="i-lucide-rewind"
                :loading="rollingBack === v.version"
                @click="rollback(v.version)"
              >Rollback</UButton>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="m-0 text-sm text-(--ui-text-muted)">No deploys yet.</p>
    </section>
  </div>
</template>
