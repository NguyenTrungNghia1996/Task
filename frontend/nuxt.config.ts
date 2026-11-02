// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  ssr: false, // Chế độ SPA theo đặc tả hệ thống
  devtools: { enabled: true },

  modules: [
    '@pinia/nuxt'
  ],

  css: [
    'ant-design-vue/dist/reset.css'
  ],

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1'
    }
  },

  nitro: {
    devProxy: {
      '/api/v1': {
        target: 'http://localhost:8080/api/v1',
        changeOrigin: true
      }
    }
  }
})
