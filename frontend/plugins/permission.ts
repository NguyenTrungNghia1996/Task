import { useAuthStore } from '~/stores/auth'

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.directive('can', {
    mounted(el, binding) {
      const authStore = useAuthStore()
      const requiredPerm = binding.value
      if (requiredPerm && !authStore.hasPermission(requiredPerm)) {
        el.parentNode?.removeChild(el)
      }
    }
  })
})
