import { defineStore } from 'pinia'

export interface UserInfo {
  id: string
  organization_id: string
  username: string
  full_name: string
  email: string
  roles: string[]
  permissions: string[]
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as UserInfo | null,
    accessToken: '' as string,
    refreshToken: '' as string,
    isLoading: false as boolean,
  }),
  getters: {
    isAuthenticated: (state) => !!state.accessToken,
    permissions: (state) => state.user?.permissions || [],
    roles: (state) => state.user?.roles || [],
  },
  actions: {
    hasPermission(code: string): boolean {
      if (this.roles.includes('system_admin')) return true
      return this.permissions.includes(code)
    },
    setAuth(accessToken: string, refreshToken: string, user: UserInfo) {
      this.accessToken = accessToken
      this.refreshToken = refreshToken
      this.user = user
      if (typeof window !== 'undefined') {
        localStorage.setItem('access_token', accessToken)
        localStorage.setItem('refresh_token', refreshToken)
        localStorage.setItem('user_info', JSON.stringify(user))
      }
    },
    initFromStorage() {
      if (typeof window !== 'undefined') {
        const at = localStorage.getItem('access_token')
        const rt = localStorage.getItem('refresh_token')
        const u = localStorage.getItem('user_info')
        if (at) this.accessToken = at
        if (rt) this.refreshToken = rt
        if (u) {
          try {
            this.user = JSON.parse(u)
          } catch {
            this.user = null
          }
        }
      }
    },
    logout() {
      this.accessToken = ''
      this.refreshToken = ''
      this.user = null
      if (typeof window !== 'undefined') {
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        localStorage.removeItem('user_info')
      }
    }
  }
})
