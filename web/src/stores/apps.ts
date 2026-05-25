import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'

export interface AppVolume {
  container: string
  host?: string
  named_volume?: string
}

export interface AppDomain {
  domain: string
  is_primary: boolean
  tls_state: string
}

export interface App {
  id: string
  name: string
  image_repo: string
  current_version: string
  instances: number
  deploy_strategy: string
  internal_only: boolean
  ports: number[]
  volumes: AppVolume[]
  domains: AppDomain[]
  created_at: number
  updated_at: number
}

export const useAppsStore = defineStore('apps', () => {
  const items = ref<App[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function load() {
    loading.value = true
    error.value = null
    try {
      const data = await api.get<{ apps: App[] }>('/api/v1/apps')
      items.value = data.apps ?? []
    } catch (err: any) {
      error.value = err?.error ?? 'failed to load apps'
    } finally {
      loading.value = false
    }
  }

  async function get(name: string): Promise<App> {
    const data = await api.get<{ app: App }>(`/api/v1/apps/${encodeURIComponent(name)}`)
    return data.app
  }

  async function create(body: { name: string; image_repo?: string; ports?: number[] }) {
    await api.post('/api/v1/apps', body)
    await load()
  }

  async function destroy(name: string) {
    await api.del(`/api/v1/apps/${encodeURIComponent(name)}`)
    await load()
  }

  return { items, loading, error, load, get, create, destroy }
})
