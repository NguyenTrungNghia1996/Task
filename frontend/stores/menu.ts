import { defineStore } from 'pinia'
import { useApi } from '~/composables/useApi'

export interface MenuItem {
  key: string
  label: string
  icon?: string
  path?: string
  children?: MenuItem[]
}

export const useMenuStore = defineStore('menu', {
  state: () => ({
    menus: [] as MenuItem[],
    isLoading: false,
  }),
  actions: {
    async fetchUserMenus() {
      const { apiFetch } = useApi()
      this.isLoading = true
      try {
        const res = await apiFetch<{ data: MenuItem[] }>('/menus/user-menus')
        this.menus = res.data || []
      } catch (err) {
        console.error('Failed to load user menus:', err)
        this.menus = []
      } finally {
        this.isLoading = false
      }
    }
  }
})
