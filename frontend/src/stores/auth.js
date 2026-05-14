import { defineStore } from 'pinia'
import { api } from '../api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    user: null,
    meLoading: false,
    meError: ''
  }),
  getters: {
    loggedIn: (state) => Boolean(state.token),
    isAdmin: (state) => state.user?.role === 'admin'
  },
  actions: {
    async login(payload) {
      const res = await api.post('/auth/login', payload)
      this.token = res.data.token
      this.user = res.data.user
      localStorage.setItem('token', this.token)
    },
    async register(payload) {
      await api.post('/auth/register', payload)
    },
    async loadMe() {
      if (!this.token) return
      if (this.meLoading) return
      this.meLoading = true
      this.meError = ''
      try {
        const res = await getMeWithRetry()
        this.user = res.data
      } catch (err) {
        if (err.authExpired) {
          this.logout()
          throw err
        }
        this.meError = err.message || '账号信息暂时不可用，请稍后刷新重试'
      } finally {
        this.meLoading = false
      }
    },
    logout() {
      this.token = ''
      this.user = null
      this.meError = ''
      localStorage.removeItem('token')
    }
  }
})

async function getMeWithRetry() {
  try {
    return await api.get('/auth/me')
  } catch (err) {
    if (!isRecoverableMeError(err)) throw err
    await delay(350)
    return api.get('/auth/me')
  }
}

function isRecoverableMeError(err) {
  if (err?.authExpired) return false
  if (!err?.status) return true
  if (err.status >= 500) return true
  return err.status === 401 && err.rawMessage === 'user not found'
}

function delay(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}
