import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'

export interface User {
  id: string
  username: string
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const bootstrapped = ref(false)
  const setupComplete = ref<boolean | null>(null)
  const error = ref<string | null>(null)

  async function loadStatus() {
    const data = await api.get<{ setup_complete: boolean }>('/api/v1/auth/status')
    setupComplete.value = data.setup_complete
    return data.setup_complete
  }

  async function refresh() {
    try {
      const data = await api.get<{ user: User }>('/api/v1/auth/me')
      user.value = data.user
    } catch {
      user.value = null
    } finally {
      bootstrapped.value = true
    }
  }

  async function setup(username: string, password: string) {
    error.value = null
    try {
      await api.post('/api/v1/auth/setup', { username, password })
      setupComplete.value = true
      await login(username, password)
    } catch (err: any) {
      error.value = err?.message ?? err?.error ?? 'setup failed'
      throw err
    }
  }

  async function login(username: string, password: string) {
    error.value = null
    try {
      const data = await api.post<{ user: User }>('/api/v1/auth/login', { username, password })
      user.value = data.user
    } catch (err: any) {
      error.value = err?.error ?? 'login failed'
      throw err
    }
  }

  async function logout() {
    await api.post('/api/v1/auth/logout')
    user.value = null
  }

  return { user, error, bootstrapped, setupComplete, loadStatus, setup, refresh, login, logout }
})
