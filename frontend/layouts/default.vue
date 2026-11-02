<template>
  <a-layout style="min-height: 100vh">
    <!-- Sidebar Menu Động Chuẩn Ant Design -->
    <a-layout-sider
      v-model:collapsed="collapsed"
      collapsible
      theme="light"
      width="250"
      style="box-shadow: 2px 0 8px 0 rgba(29,35,41,.05); z-index: 10;"
    >
      <div class="logo">
        <span v-if="!collapsed" class="logo-text">QUẢN LÝ CÔNG VIỆC</span>
        <span v-else class="logo-text">QLCV</span>
      </div>

      <a-menu
        v-model:selectedKeys="selectedKeys"
        v-model:openKeys="openKeys"
        mode="inline"
        :items="menuItems"
        @click="handleMenuClick"
      />
    </a-layout-sider>

    <a-layout>
      <!-- Header -->
      <a-layout-header style="background: #fff; padding: 0 24px; display: flex; justify-content: flex-end; align-items: center; border-bottom: 1px solid #f0f0f0;">
        <a-dropdown :trigger="['click']">
          <div style="cursor: pointer; display: flex; align-items: center; gap: 8px;">
            <a-avatar style="background-color: #1890ff">
              {{ authStore.user?.full_name?.charAt(0) || 'U' }}
            </a-avatar>
            <div style="line-height: 1.2;">
              <div style="font-weight: 600;">{{ authStore.user?.full_name || authStore.user?.username }}</div>
              <div style="font-size: 11px; color: #8c8c8c;">
                {{ authStore.user?.roles?.[0] || 'User' }}
              </div>
            </div>
            <DownOutlined style="font-size: 10px; color: #8c8c8c;" />
          </div>

          <template #overlay>
            <a-menu>
              <a-menu-item key="user-info" disabled>
                <span>Tài khoản: <strong>{{ authStore.user?.username }}</strong></span>
              </a-menu-item>
              <a-menu-divider />
              <a-menu-item key="logout" @click="handleLogout">
                <LogoutOutlined style="color: #ff4d4f; margin-right: 8px;" />
                <span style="color: #ff4d4f;">Đăng xuất</span>
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-layout-header>

      <!-- Nội dung trang -->
      <a-layout-content style="margin: 24px 24px 0; overflow: initial;">
        <div style="padding: 24px; background: #fff; border-radius: 6px; min-height: calc(100vh - 160px);">
          <slot />
        </div>
      </a-layout-content>

      <a-layout-footer style="text-align: center; color: #8c8c8c; font-size: 12px;">
        Hệ Thống Quản Lý Nhiệm Vụ Nhiều Cấp ©2026
      </a-layout-footer>
    </a-layout>
  </a-layout-sider>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import * as AntdIcons from '@ant-design/icons-vue'
import { DownOutlined, LogoutOutlined } from '@ant-design/icons-vue'
import { useAuthStore } from '~/stores/auth'
import { useMenuStore, type MenuItem } from '~/stores/menu'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const menuStore = useMenuStore()

const collapsed = ref(false)
const selectedKeys = ref<string[]>([route.path])
const openKeys = ref<string[]>(['/' + route.path.split('/')[1]])

watch(() => route.path, (newPath) => {
  selectedKeys.value = [newPath]
})

onMounted(async () => {
  authStore.initFromStorage()
  if (authStore.isAuthenticated) {
    await menuStore.fetchUserMenus()
  }
})

// Chuyển chuỗi tên Icon (vd: "DashboardOutlined") thành VNode
const renderIcon = (iconName?: string) => {
  if (!iconName || !(iconName in AntdIcons)) return null
  return () => h((AntdIcons as any)[iconName])
}

// Đệ quy ánh xạ Menu từ Backend sang định dạng của a-menu
const transformMenus = (items: MenuItem[]): any[] => {
  return items.map(item => ({
    key: item.path || item.key,
    label: item.label,
    icon: renderIcon(item.icon),
    children: item.children && item.children.length > 0 ? transformMenus(item.children) : undefined,
    path: item.path
  }))
}

const menuItems = computed(() => transformMenus(menuStore.menus))

const handleMenuClick = ({ key }: { key: string }) => {
  if (key && key.startsWith('/')) {
    router.push(key)
  }
}

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid #f0f0f0;
}
.logo-text {
  font-weight: 700;
  font-size: 15px;
  color: #1890ff;
  letter-spacing: 0.5px;
  white-space: nowrap;
}
</style>
