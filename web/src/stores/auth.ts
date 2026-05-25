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
  const error = ref<string | null>(null)

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

  return { user, error, bootstrapped, refresh, login, logout }
})
