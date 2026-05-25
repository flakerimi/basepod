<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
  const e = await api.get<{ env: Record<string, string> }>(`/api/v1/apps/${props.name}/env`)
  env.value = e.env ?? {}
  await loadGit()
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
