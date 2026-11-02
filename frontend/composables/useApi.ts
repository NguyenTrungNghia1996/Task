import { useAuthStore } from '~/stores/auth'

export const useApi = () => {
  const config = useRuntimeConfig()
  const authStore = useAuthStore()

  const apiFetch = async <T = any>(
    endpoint: string,
    options: Parameters<typeof $fetch>[1] = {}
  ): Promise<T> => {
    const baseURL = config.public.apiBase || 'http://localhost:8080/api/v1'
    const token = authStore.accessToken || (typeof window !== 'undefined' ? localStorage.getItem('access_token') : '')

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    try {
      return await $fetch<T>(endpoint, {
        baseURL,
        ...options,
        headers,
      })
    } catch (err: any) {
      // Nếu 401 UNAUTHORIZED -> thử refresh token
      if (err.statusCode === 401 && authStore.refreshToken) {
        try {
          const refreshRes: any = await $fetch('/auth/refresh', {
            baseURL,
            method: 'POST',
            body: { refresh_token: authStore.refreshToken }
          })
          if (refreshRes?.data) {
            authStore.setAuth(
              refreshRes.data.access_token,
              refreshRes.data.refresh_token,
              refreshRes.data.user
            )
            // Thử gọi lại request ban đầu với token mới
            headers['Authorization'] = `Bearer ${refreshRes.data.access_token}`
            return await $fetch<T>(endpoint, {
              baseURL,
              ...options,
              headers,
            })
          }
        } catch (refreshErr) {
          authStore.logout()
          if (typeof window !== 'undefined') {
            navigateTo('/login')
          }
          throw refreshErr
        }
      }
      throw err
    }
  }

  return {
    apiFetch,
  }
}
