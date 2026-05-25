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
const git = ref<GitConfig>({ url: '', branch: 'main', dockerfile: 'Dockerfile', has_credential: false, has_webhook: false, webhook_url: '' })
const gitToken = ref('')
const gitUser = ref('')
const gitPass = ref('')
const gitAuthMode = ref<'token' | 'userpass'>('token')
const gitSaving = ref(false)
const gitDeploying = ref(false)
const gitWebhookRevealed = ref<string | null>(null)
const gitWebhookURL = ref('')

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
    git.value = g
    gitWebhookURL.value = g.webhook_url
  } catch { /* fresh app */ }
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
        { label: 'Git', value: 'git', icon: 'i-lucide-git-branch' },
        { label: 'Logs', value: 'logs' },
        { label: 'Versions', value: 'versions' },
      ]"
    />

    <section v-if="tab === 'overview'" class="overview-tab">
      <!-- Status banner -->
      <div class="status-card">
        <div class="status-main">
          <span class="dot" :data-state="app.current_version ? 'running' : 'idle'"></span>
          <div>
            <div class="status-label">{{ app.current_version ? 'Deployed' : 'Not deployed' }}</div>
            <div class="status-sub muted">
              <code v-if="app.image_repo">{{ app.image_repo }}</code>
              <span v-else>no image yet</span>
              <span v-if="app.current_version"> · <code>{{ app.current_version }}</code></span>
            </div>
          </div>
        </div>
        <div class="status-chips">
          <span class="chip">{{ app.deploy_strategy }}</span>
          <span class="chip" v-if="app.instances > 1">×{{ app.instances }}</span>
          <span class="chip" v-if="app.internal_only">internal</span>
          <span class="chip" v-if="app.memory_mb">{{ app.memory_mb }} MB</span>
          <span class="chip" v-if="app.cpu_pct">{{ app.cpu_pct }}% CPU</span>
        </div>
      </div>

      <!-- Two columns -->
      <div class="two-col">
        <!-- LEFT: configuration form -->
        <div class="ov-card">
          <div class="card-head">
            <h3>Configuration</h3>
            <span v-if="cfgSavedMsg" class="muted small">{{ cfgSavedMsg }}</span>
          </div>
          <div class="form-grid">
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
            <UFormField label="Healthcheck path" class="span-2">
              <UInput v-model="cfg.healthcheck_path" placeholder="/healthz" />
            </UFormField>
            <div class="span-2 switch-row">
              <div>
                <div class="switch-label">Internal only</div>
                <div class="muted small">Excluded from public Caddy routing</div>
              </div>
              <USwitch v-model="cfg.internal_only" />
            </div>
          </div>
          <div class="card-foot">
            <UButton :loading="cfgSaving" :disabled="!cfgDirty" @click="saveCfg">Save changes</UButton>
            <UButton v-if="cfgDirty" color="neutral" variant="ghost" @click="resetCfg">Reset</UButton>
          </div>
        </div>

        <!-- RIGHT: ports + volumes stacked -->
        <div class="right-col">
          <div class="ov-card compact">
            <div class="card-head">
              <h3>Ports</h3>
              <span class="muted small">applied on redeploy</span>
            </div>
            <ul class="chips-list" v-if="app.ports.length">
              <li v-for="p in app.ports" :key="p">
                <code>{{ p }}</code>
                <button class="chip-x" @click="removePort(p)" aria-label="remove">×</button>
              </li>
            </ul>
            <p v-else class="muted small empty">No ports defined.</p>
            <div class="add-inline">
              <UInput v-model.number="newPort" type="number" min="1" max="65535" placeholder="3000" size="sm" />
              <UButton size="sm" icon="i-lucide-plus" :disabled="!newPort" @click="addPort">Add</UButton>
            </div>
          </div>

          <div class="ov-card compact">
            <div class="card-head">
              <h3>Volumes</h3>
              <span class="muted small"><code>~/</code> expands on save</span>
            </div>
            <ul class="vol-list" v-if="app.volumes.length">
              <li v-for="v in app.volumes" :key="v.container">
                <div class="vol-line">
                  <code class="vol-c">{{ v.container }}</code>
                  <span class="muted arrow">←</span>
                  <code class="vol-h">{{ v.host || v.named_volume }}</code>
                </div>
                <button class="chip-x" @click="removeVolume(v.container)" aria-label="remove">×</button>
              </li>
            </ul>
            <p v-else class="muted small empty">No volumes mounted.</p>
            <div class="add-inline vol-add">
              <UInput v-model="newVolContainer" placeholder="/data" size="sm" />
              <UInput v-model="newVolHost" placeholder="~/BasePodData/myapp/data" size="sm" />
              <UButton size="sm" icon="i-lucide-plus" :disabled="!newVolContainer || !newVolHost" @click="addVolume">Add</UButton>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section v-else-if="tab === 'env'" class="section env-tab">
      <div class="env-head">
        <h3 class="section-title">Environment variables</h3>
        <URadioGroup
          v-model="envMode"
          :items="[{ label: 'Rows', value: 'rows' }, { label: 'Bulk (.env)', value: 'bulk' }]"
          orientation="horizontal"
          size="sm"
          @update:model-value="(v: string) => v === 'bulk' && syncBulkFromRows()"
        />
      </div>

      <template v-if="envMode === 'rows'">
        <div v-for="(value, key) in env" :key="String(key)" class="env-row">
          <UInput :model-value="String(key)" disabled class="k" />
          <UInput
            v-model="env[String(key)]"
            :type="envRevealed[String(key)] || !isSecretKey(String(key)) ? 'text' : 'password'"
            class="v"
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
        <div class="env-row">
          <UInput placeholder="NEW_KEY" v-model="newKey" class="k" />
          <UInput placeholder="value" v-model="newVal" class="v" />
          <UButton size="xs" icon="i-lucide-plus" :disabled="!newKey" @click="addRow" />
        </div>
      </template>

      <template v-else>
        <UTextarea
          v-model="envBulk"
          :rows="14"
          autoresize
          placeholder="# Paste like a .env file
NODE_ENV=production
DATABASE_URL=postgres://..."
          class="bulk"
        />
      </template>

      <div class="env-actions">
        <UCheckbox v-model="envRestartOnSave" label="Restart container after save" />
        <div class="grow" />
        <span v-if="envSavedMsg" class="muted small">{{ envSavedMsg }}</span>
        <UButton :loading="envSaving" @click="saveEnv">Save env</UButton>
      </div>
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

    <section v-else-if="tab === 'git'" class="section git-tab">
      <h3 class="section-title">Deploy from Git</h3>
      <p class="muted">
        Build straight from a GitHub, GitLab, or Bitbucket repository. After save,
        rotate a webhook secret to enable push-to-deploy.
      </p>

      <div class="grid">
        <UFormField label="Repository URL">
          <UInput v-model="git.url" placeholder="https://github.com/owner/repo" />
        </UFormField>
        <UFormField label="Branch">
          <UInput v-model="git.branch" placeholder="main" />
        </UFormField>
        <UFormField label="Dockerfile path">
          <UInput v-model="git.dockerfile" placeholder="Dockerfile" />
        </UFormField>
      </div>

      <h4 class="sub">Authentication</h4>
      <p class="muted small">
        Public repos: leave blank.
        Private GitHub: paste a PAT.
        GitLab/Bitbucket: use username + password (or app password / project token).
      </p>
      <URadioGroup
        v-model="gitAuthMode"
        :items="[
          { label: 'GitHub PAT / token', value: 'token' },
          { label: 'Username + password', value: 'userpass' },
        ]"
        orientation="horizontal"
      />
      <div v-if="gitAuthMode === 'token'" class="grid">
        <UFormField :label="git.has_credential ? 'Replace token (leave blank to keep existing)' : 'Token'">
          <UInput v-model="gitToken" type="password" placeholder="ghp_xxx" />
        </UFormField>
      </div>
      <div v-else class="grid">
        <UFormField label="Username">
          <UInput v-model="gitUser" placeholder="me / oauth2" />
        </UFormField>
        <UFormField :label="git.has_credential ? 'Replace password' : 'Password / app password'">
          <UInput v-model="gitPass" type="password" />
        </UFormField>
      </div>

      <div class="actions-row">
        <UButton :loading="gitSaving" @click="saveGit" :disabled="!git.url">Save</UButton>
        <UButton
          color="neutral"
          variant="outline"
          icon="i-lucide-rocket"
          :loading="gitDeploying"
          :disabled="!git.url"
          @click="deployFromGit"
        >
          Force build
        </UButton>
      </div>

      <hr class="sep" />

      <h4 class="sub">Webhook</h4>
      <p class="muted small">
        GitHub / GitLab will POST here on every push. We verify the HMAC signature
        against the secret below before triggering a deploy.
      </p>

      <div class="webhook-box" v-if="git.webhook_url">
        <div class="kv-row">
          <span class="label">Payload URL</span>
          <code class="value">{{ gitWebhookURL || git.webhook_url }}</code>
          <UButton size="xs" variant="ghost" icon="i-lucide-copy" @click="copy(gitWebhookURL || git.webhook_url)" />
        </div>
        <div class="kv-row" v-if="gitWebhookRevealed">
          <span class="label">Secret (shown once)</span>
          <code class="value secret">{{ gitWebhookRevealed }}</code>
          <UButton size="xs" variant="ghost" icon="i-lucide-copy" @click="copy(gitWebhookRevealed)" />
        </div>
        <div class="kv-row" v-else-if="git.has_webhook">
          <span class="label">Secret</span>
          <span class="muted">configured — rotate below to view a new one</span>
        </div>
      </div>

      <div class="actions-row">
        <UButton variant="outline" icon="i-lucide-key" @click="rotateWebhook">
          {{ git.has_webhook ? 'Rotate webhook secret' : 'Generate webhook secret' }}
        </UButton>
      </div>

      <details class="instructions">
        <summary>Connect GitHub</summary>
        <ol>
          <li>In your repo: Settings → Webhooks → Add webhook</li>
          <li>Payload URL: paste the URL above</li>
          <li>Content type: <code>application/json</code></li>
          <li>Secret: paste the secret above (shown only after rotate)</li>
          <li>Events: <strong>Just the push event</strong></li>
          <li>Save. Next push to <code>{{ git.branch || 'main' }}</code> triggers a deploy.</li>
        </ol>
      </details>
    </section>

    <section v-else-if="tab === 'logs'" class="section">
      <div class="logs-bar">
        <UButton v-if="!stopLogs" icon="i-lucide-play" @click="startLogs">Stream</UButton>
        <UButton v-else color="neutral" icon="i-lucide-square" @click="stopLogStream">Stop</UButton>
      </div>
      <pre class="logs">{{ logs.join('\n') || '— start streaming to see container logs —' }}</pre>
    </section>

    <section v-else-if="tab === 'versions'" class="section versions-tab">
      <h3 class="section-title">Deploy history</h3>
      <p class="muted small">Last 5 versions are kept. Rollback re-deploys the selected image tag.</p>
      <table v-if="versions.length" class="versions">
        <thead>
          <tr>
            <th>Version</th>
            <th>Image</th>
            <th>Status</th>
            <th>Deployed</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="v in versions" :key="v.id" :class="{ current: v.version === app.current_version }">
            <td><code>{{ v.version }}</code><span v-if="v.version === app.current_version" class="badge">current</span></td>
            <td><code>{{ v.image_tag || '—' }}</code></td>
            <td><span class="status" :data-status="v.status">{{ v.status }}</span></td>
            <td>{{ formatTs(v.deployed_at) }}</td>
            <td>
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
      <p v-else class="muted">No deploys yet.</p>
    </section>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 1.5rem; }
.head { display: flex; justify-content: space-between; align-items: start; }
.head h1 { margin: 0; }
.muted { color: var(--ui-text-muted); }
.muted.small { font-size: 0.85rem; margin-top: -0.25rem; }
.section { background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); border-radius: 1rem; padding: 1.25rem; }
.section-title { margin: 0 0 0.5rem; font-size: 1.05rem; }
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

/* overview tab — redesigned */
.overview-tab { display: flex; flex-direction: column; gap: 1rem; }

/* status banner */
.status-card {
  background: var(--ui-bg-elevated);
  border: 1px solid var(--ui-border);
  border-radius: 1rem;
  padding: 1rem 1.25rem;
  display: flex; align-items: center; justify-content: space-between;
  gap: 1rem; flex-wrap: wrap;
}
.status-main { display: flex; align-items: center; gap: 0.85rem; }
.dot {
  width: 10px; height: 10px; border-radius: 50%;
  background: var(--ui-border);
  box-shadow: 0 0 0 4px color-mix(in oklch, var(--ui-border) 30%, transparent);
}
.dot[data-state="running"] {
  background: #16a34a;
  box-shadow: 0 0 0 4px color-mix(in oklch, #16a34a 25%, transparent);
}
.status-label { font-weight: 600; font-size: 0.95rem; }
.status-sub { font-size: 0.85rem; }
.status-sub code { background: none; padding: 0; }
.status-chips { display: flex; gap: 0.4rem; flex-wrap: wrap; }
.chip {
  font-size: 0.72rem;
  padding: 0.15rem 0.55rem;
  background: var(--ui-bg-muted);
  border: 1px solid var(--ui-border);
  border-radius: 1rem;
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  color: var(--ui-text-muted);
}

/* two-column grid */
.two-col {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: 1rem;
  align-items: start;
}
.right-col { display: flex; flex-direction: column; gap: 1rem; }

/* cards */
.ov-card {
  background: var(--ui-bg-elevated);
  border: 1px solid var(--ui-border);
  border-radius: 1rem;
  padding: 1.25rem;
  display: flex; flex-direction: column; gap: 1rem;
}
.ov-card.compact { padding: 1rem 1.25rem; gap: 0.75rem; }
.card-head { display: flex; align-items: baseline; justify-content: space-between; gap: 0.75rem; }
.card-head h3 { margin: 0; font-size: 1rem; }
.card-foot {
  display: flex; gap: 0.5rem; align-items: center;
  padding-top: 0.75rem; border-top: 1px solid var(--ui-border);
}
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem 1rem; }
.form-grid .span-2 { grid-column: 1 / -1; }
.switch-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0.5rem 0.75rem;
  background: var(--ui-bg-muted);
  border-radius: 0.6rem;
}
.switch-label { font-size: 0.85rem; font-weight: 500; }

/* chips list (ports) */
.chips-list {
  list-style: none; padding: 0; margin: 0;
  display: flex; flex-wrap: wrap; gap: 0.4rem;
}
.chips-list li {
  display: inline-flex; align-items: center; gap: 0.4rem;
  padding: 0.3rem 0.55rem;
  background: var(--ui-bg-muted);
  border-radius: 0.45rem;
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  font-size: 0.85rem;
}
.chip-x {
  background: none; border: 0; cursor: pointer;
  color: var(--ui-text-muted); font-size: 1.05rem; line-height: 1;
  padding: 0; width: 1rem; height: 1rem;
}
.chip-x:hover { color: #dc2626; }

/* volumes list */
.vol-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 0.4rem; }
.vol-list li {
  display: flex; align-items: center; gap: 0.6rem;
  padding: 0.5rem 0.75rem;
  background: var(--ui-bg-muted);
  border-radius: 0.6rem;
  font-size: 0.85rem;
}
.vol-line { display: flex; align-items: center; gap: 0.5rem; min-width: 0; flex: 1; }
.vol-c, .vol-h { font-family: ui-monospace, "SF Mono", Menlo, monospace; font-size: 0.82rem; }
.vol-h { color: var(--ui-text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.arrow { font-size: 0.85rem; }

.add-inline { display: flex; gap: 0.4rem; align-items: stretch; }
.add-inline.vol-add { display: grid; grid-template-columns: 1fr 1.6fr auto; }
.empty { padding: 0.5rem 0; margin: 0; }

@media (max-width: 1000px) {
  .two-col { grid-template-columns: 1fr; }
  .form-grid { grid-template-columns: 1fr; }
}

/* versions tab */
.versions-tab { display: flex; flex-direction: column; gap: 0.75rem; }
.versions { width: 100%; border-collapse: collapse; }
.versions th, .versions td { padding: 0.6rem 0.75rem; text-align: left; border-bottom: 1px solid var(--ui-border); }
.versions th { color: var(--ui-text-muted); font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; }
.versions tr.current { background: color-mix(in oklch, var(--color-primary-500) 6%, transparent); }
.versions code { font-size: 0.85rem; }
.versions .badge {
  display: inline-block;
  margin-left: 0.5rem;
  padding: 0.05rem 0.4rem;
  background: var(--color-primary-500);
  color: white;
  border-radius: 0.25rem;
  font-size: 0.7rem;
}
.versions .status { font-size: 0.85rem; }
.versions .status[data-status="succeeded"] { color: #16a34a; }
.versions .status[data-status="failed"] { color: #dc2626; }
.versions .status[data-status="deploying"], .versions .status[data-status="building"], .versions .status[data-status="cloning"] { color: var(--color-primary-600); }

/* env tab */
.env-tab { display: flex; flex-direction: column; gap: 0.75rem; }
.env-head { display: flex; align-items: center; justify-content: space-between; }
.env-row {
  display: grid;
  grid-template-columns: 1fr 2fr auto auto;
  gap: 0.5rem;
  align-items: center;
}
.env-row .k { font-family: ui-monospace, "SF Mono", Menlo, monospace; }
.bulk :deep(textarea) {
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  font-size: 0.875rem;
}
.env-actions {
  display: flex; align-items: center; gap: 0.75rem;
  padding-top: 0.5rem; border-top: 1px solid var(--ui-border);
}
.grow { flex: 1; }

/* git tab */
.git-tab { display: flex; flex-direction: column; gap: 1rem; }
.git-tab .sub { margin: 0.5rem 0 0; font-size: 0.95rem; }
.git-tab .grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 0.75rem; }
.git-tab .grid > :first-child:nth-last-child(1) { grid-column: 1 / -1; }
.actions-row { display: flex; gap: 0.5rem; }
.sep { border: 0; border-top: 1px solid var(--ui-border); margin: 0.5rem 0; }
.webhook-box {
  background: var(--ui-bg);
  border: 1px solid var(--ui-border);
  border-radius: 0.75rem;
  padding: 1rem;
  display: flex; flex-direction: column; gap: 0.5rem;
}
.kv-row {
  display: grid; grid-template-columns: 180px 1fr auto; gap: 0.75rem; align-items: center;
}
.kv-row .label { color: var(--ui-text-muted); font-size: 0.85rem; }
.kv-row .value {
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  font-size: 0.85rem;
  background: var(--ui-bg-muted);
  padding: 0.25rem 0.5rem;
  border-radius: 0.4rem;
  word-break: break-all;
}
.kv-row .value.secret { color: var(--color-primary-600); }
.instructions {
  background: var(--ui-bg);
  border: 1px solid var(--ui-border);
  border-radius: 0.75rem;
  padding: 0.75rem 1rem;
}
.instructions summary { cursor: pointer; font-weight: 500; }
.instructions ol { margin: 0.75rem 0 0; padding-left: 1.25rem; line-height: 1.7; }
.instructions code {
  background: var(--ui-bg-muted);
  padding: 0.1rem 0.35rem;
  border-radius: 0.25rem;
  font-size: 0.85em;
}
</style>
