<template>
  <div>
    <div style="margin-bottom: 24px;">
      <h2 style="margin: 0; font-size: 22px; font-weight: 700;">Bảng Tổng Quan</h2>
      <p style="color: #8c8c8c; margin: 4px 0 0;">Chào mừng bạn quay trở lại, <strong>{{ authStore.user?.full_name }}</strong>!</p>
    </div>

    <!-- Thông tin quyền hạn hiện tại -->
    <a-alert
      type="info"
      show-icon
      style="margin-bottom: 24px"
    >
      <template #message>
        <span>Vai trò: <a-tag color="blue">{{ authStore.roles.join(', ') }}</a-tag></span>
        <span style="margin-left: 16px;">Số quyền được cấp: <strong>{{ authStore.permissions.length }}</strong></span>
      </template>
    </a-alert>

    <!-- Thống kê nhanh -->
    <a-row :gutter="[16, 16]">
      <a-col :xs="24" :sm="12" :md="6">
        <a-card>
          <a-statistic
            title="Tổng công việc"
            :value="taskStats.total"
            :value-style="{ color: '#1890ff' }"
          >
            <template #prefix>
              <ProjectOutlined />
            </template>
          </a-statistic>
        </a-card>
      </a-col>

      <a-col :xs="24" :sm="12" :md="6">
        <a-card>
          <a-statistic
            title="Đang thực hiện"
            :value="taskStats.inProgress"
            :value-style="{ color: '#faad14' }"
          >
            <template #prefix>
              <SyncOutlined spin />
            </template>
          </a-statistic>
        </a-card>
      </a-col>

      <a-col :xs="24" :sm="12" :md="6">
        <a-card>
          <a-statistic
            title="Đã hoàn thành"
            :value="taskStats.completed"
            :value-style="{ color: '#52c41a' }"
          >
            <template #prefix>
              <CheckCircleOutlined />
            </template>
          </a-statistic>
        </a-card>
      </a-col>

      <a-col :xs="24" :sm="12" :md="6">
        <a-card>
          <a-statistic
            title="Quá hạn"
            :value="taskStats.overdue"
            :value-style="{ color: '#ff4d4f' }"
          >
            <template #prefix>
              <ClockCircleOutlined />
            </template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <!-- Nút hành động nhanh có áp dụng v-can -->
    <div style="margin-top: 32px;">
      <h3 style="margin-bottom: 16px; font-weight: 600;">Thao tác nhanh</h3>
      <a-space wrap size="middle">
        <!-- Nút Tạo nhiệm vụ mới: Chỉ hiển thị nếu có quyền tasks:create -->
        <a-button type="primary" size="large" v-can="'tasks:create'" @click="navigateTo('/tasks/create')">
          <PlusOutlined /> Giao nhiệm vụ mới
        </a-button>

        <!-- Nút Xem cây tổ chức: Chỉ hiển thị nếu có quyền org:view_tree -->
        <a-button size="large" v-can="'org:view_tree'" @click="navigateTo('/organization')">
          <ClusterOutlined /> Sơ đồ tổ chức
        </a-button>

        <!-- Nút Cấu hình hệ thống: Chỉ hiển thị nếu có quyền system:settings -->
        <a-button size="large" v-can="'system:settings'" @click="navigateTo('/admin')">
          <SettingOutlined /> Cấu hình hệ thống
        </a-button>
      </a-space>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted } from 'vue'
import {
  ProjectOutlined,
  SyncOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  PlusOutlined,
  ClusterOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue'
import { useAuthStore } from '~/stores/auth'
import { useApi } from '~/composables/useApi'

const authStore = useAuthStore()
const { apiFetch } = useApi()

const taskStats = reactive({
  total: 0,
  inProgress: 0,
  completed: 0,
  overdue: 0,
})

onMounted(async () => {
  try {
    const res: any = await apiFetch('/tasks?page=1&limit=10')
    if (res?.data) {
      taskStats.total = res.meta?.total || res.data?.length || 0
      taskStats.inProgress = res.data?.filter((t: any) => t.status === 'in_progress').length || 0
      taskStats.completed = res.data?.filter((t: any) => t.status === 'completed').length || 0
    }
  } catch {
    // Demo fallback values
    taskStats.total = 12
    taskStats.inProgress = 5
    taskStats.completed = 6
    taskStats.overdue = 1
  }
})
</script>
