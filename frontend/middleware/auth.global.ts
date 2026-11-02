import { useAuthStore } from '~/stores/auth'

export default defineNuxtRouteMiddleware((to) => {
  // Bỏ qua route login
  if (to.path === '/login') {
    return
  }

  const authStore = useAuthStore()

  // Khôi phục session từ localStorage nếu chưa nạp
  if (!authStore.isAuthenticated) {
    authStore.initFromStorage()
  }

  // Nếu vẫn chưa đăng nhập -> chuyển về /login
  if (!authStore.isAuthenticated) {
    return navigateTo('/login')
  }

  // Kiểm tra quyền theo meta của route nếu có
  const requiredPerm = to.meta.requiredPermission as string | undefined
  if (requiredPerm && !authStore.hasPermission(requiredPerm)) {
    return navigateTo('/403')
  }
})
