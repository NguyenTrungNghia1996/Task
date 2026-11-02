<template>
  <div class="login-wrapper">
    <a-card class="login-card" :bordered="false">
      <div class="header">
        <h2 class="title">HỆ THỐNG QUẢN LÝ CÔNG VIỆC</h2>
        <p class="subtitle">Đăng nhập tài khoản để tiếp tục</p>
      </div>

      <a-alert
        v-if="errorMessage"
        :message="errorMessage"
        type="error"
        show-icon
        closable
        style="margin-bottom: 20px"
        @close="errorMessage = ''"
      />

      <a-form
        :model="formState"
        name="loginForm"
        layout="vertical"
        @finish="handleLogin"
      >
        <a-form-item
          label="Tên đăng nhập"
          name="username"
          :rules="[{ required: true, message: 'Vui lòng nhập tên đăng nhập!' }]"
        >
          <a-input
            v-model:value="formState.username"
            size="large"
            placeholder="vd: admin, truongphong, nhanvien"
          >
            <template #prefix>
              <UserOutlined style="color: rgba(0, 0, 0, 0.25)" />
            </template>
          </a-input>
        </a-form-item>

        <a-form-item
          label="Mật khẩu"
          name="password"
          :rules="[{ required: true, message: 'Vui lòng nhập mật khẩu!' }]"
        >
          <a-input-password
            v-model:value="formState.password"
            size="large"
            placeholder="Mật khẩu"
          >
            <template #prefix>
              <LockOutlined style="color: rgba(0, 0, 0, 0.25)" />
            </template>
          </a-input-password>
        </a-form-item>

        <a-form-item>
          <a-button
            type="primary"
            html-type="submit"
            size="large"
            block
            :loading="loading"
          >
            Đăng nhập
          </a-button>
        </a-form-item>
      </a-form>

      <div class="test-accounts">
        <div style="font-size: 12px; font-weight: 600; color: #8c8c8c; margin-bottom: 6px;">TÀI KHOẢN MẪU (Pass: password123)</div>
        <div style="display: flex; gap: 8px; flex-wrap: wrap;">
          <a-tag color="blue" style="cursor: pointer" @click="quickFill('admin')">admin (SA)</a-tag>
          <a-tag color="green" style="cursor: pointer" @click="quickFill('commander')">commander (Chỉ huy)</a-tag>
          <a-tag color="purple" style="cursor: pointer" @click="quickFill('truongphong')">truongphong</a-tag>
          <a-tag color="orange" style="cursor: pointer" @click="quickFill('nhanvien')">nhanvien</a-tag>
        </div>
      </div>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { UserOutlined, LockOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { useAuthStore } from '~/stores/auth'
import { useMenuStore } from '~/stores/menu'
import { useApi } from '~/composables/useApi'

definePageMeta({
  layout: false // Không dùng layout sidebar cho trang login
})

const router = useRouter()
const authStore = useAuthStore()
const menuStore = useMenuStore()
const { apiFetch } = useApi()

const formState = reactive({
  username: 'admin',
  password: 'password123',
})

const loading = ref(false)
const errorMessage = ref('')

const quickFill = (user: string) => {
  formState.username = user
  formState.password = 'password123'
}

const handleLogin = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const res: any = await apiFetch('/auth/login', {
      method: 'POST',
      body: {
        username: formState.username,
        password: formState.password,
      }
    })

    if (res?.data) {
      authStore.setAuth(
        res.data.access_token,
        res.data.refresh_token,
        res.data.user
      )
      message.success('Đăng nhập thành công!')
      // Tải trước menu cho user
      await menuStore.fetchUserMenus()
      router.push('/dashboard')
    }
  } catch (err: any) {
    const msg = err.data?.error?.message || err.message || 'Đăng nhập không thành công'
    errorMessage.value = msg
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrapper {
  min-height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #1890ff 0%, #001529 100%);
  padding: 16px;
}

.login-card {
  width: 100%;
  max-width: 420px;
  border-radius: 10px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
}

.header {
  text-align: center;
  margin-bottom: 24px;
}

.title {
  font-size: 20px;
  font-weight: 700;
  color: #001529;
  margin-bottom: 4px;
}

.subtitle {
  font-size: 13px;
  color: #8c8c8c;
  margin: 0;
}

.test-accounts {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px dashed #f0f0f0;
}
</style>
