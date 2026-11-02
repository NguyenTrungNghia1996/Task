# Hướng Dẫn Tích Hợp Menu Động & Phân Quyền (RBAC) Phía Frontend (Ant Design Vue)

Tài liệu này hướng dẫn chi tiết cách Frontend (Nuxt 4 / Vue 3 kết hợp thư viện **Ant Design Vue**) tích hợp với hệ thống **Menu động** và **Phân quyền theo chức năng (Fine-grained RBAC)** từ Backend.

---

## 1. Danh Sách API Backend Hỗ Trợ Frontend

| Phương thức | Endpoint | Chức năng | Dành cho ai |
|---|---|---|---|
| `GET` | `/api/v1/menus/user-menus` | Lấy cây Menu đã lọc theo quyền của user (chuẩn Ant Design) | Mọi user đã login |
| `GET` | `/api/v1/auth/me` | Lấy thông tin user hiện tại kèm mảng `roles` và `permissions` | Mọi user đã login |
| `GET` | `/api/v1/permissions/options` | Lấy danh sách quyền dạng options `{value, label, module}` cho `<a-select>` | SA (Quản trị Menu) |
| `GET` | `/api/v1/permissions` | Lấy toàn bộ quyền gom nhóm theo phân hệ cho bảng ma trận | SA (Quản lý quyền) |
| `GET` | `/api/v1/roles` | Danh sách các vai trò hệ thống | SA |
| `GET` | `/api/v1/roles/:roleId/permissions` | Xem danh sách quyền hiện tại của một vai trò | SA |
| `PUT` | `/api/v1/roles/:roleId/permissions` | Cập nhật tập quyền cho vai trò (tích chọn checkbox) | SA |
| `GET` | `/api/v1/menus` | Xem toàn bộ cây menu (kể cả menu ẩn) để cấu hình | SA |
| `POST` | `/api/v1/menus` | Thêm mục menu mới | SA |
| `PATCH` | `/api/v1/menus/:menuId` | Cập nhật menu (đổi tên, icon, đường dẫn, quyền yêu cầu) | SA |
| `DELETE` | `/api/v1/menus/:menuId` | Xóa mục menu (chỉ xóa menu không có menu con) | SA |

> **Lưu ý**: Hệ thống hỗ trợ cả 2 cách gọi có hoặc không có tiền tố `/api/v1` (ví dụ: `/menus/user-menus` hoặc `/api/v1/menus/user-menus` đều hoạt động tốt).

---

## 2. Hiển Thị Menu Sidebar Động (Chuẩn Ant Design Vue)

Backend đã định dạng sẵn dữ liệu trả về từ API `GET /api/v1/menus/user-menus` khớp 100% với thuộc tính `:items` của Ant Design `<a-menu>`.

### 2.1 Cấu trúc dữ liệu JSON Backend trả về

```json
{
  "data": [
    {
      "key": "/dashboard",
      "label": "Bảng tổng quan",
      "icon": "DashboardOutlined",
      "path": "/dashboard"
    },
    {
      "key": "/tasks",
      "label": "Quản lý công việc",
      "icon": "ProjectOutlined",
      "children": [
        {
          "key": "/tasks/all",
          "label": "Tất cả công việc",
          "icon": "UnorderedListOutlined",
          "path": "/tasks/all"
        },
        {
          "key": "/tasks/department",
          "label": "Việc phòng ban",
          "icon": "ApartmentOutlined",
          "path": "/tasks/department"
        },
        {
          "key": "/tasks/my-tasks",
          "label": "Việc của tôi",
          "icon": "CheckSquareOutlined",
          "path": "/tasks/my-tasks"
        }
      ]
    }
  ]
}
```

### 2.2 Component Vue mẫu: `AppSidebar.vue`

```vue
<template>
  <a-layout-sider v-model:collapsed="collapsed" collapsible theme="light" width="260">
    <div class="logo-box py-4 text-center font-bold text-lg text-primary">
      QUẢN LÝ CÔNG VIỆC
    </div>

    <!-- Ant Design Menu: Sử dụng trực tiếp mảng items đã map icon -->
    <a-menu
      v-model:selectedKeys="selectedKeys"
      v-model:openKeys="openKeys"
      mode="inline"
      :items="menuItems"
      @click="handleMenuClick"
    />
  </a-layout-sider>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import * as AntdIcons from '@ant-design/icons-vue'

const router = useRouter()
const route = useRoute()
const collapsed = ref(false)

const selectedKeys = ref<string[]>([route.path])
const openKeys = ref<string[]>(['/' + route.path.split('/')[1]])
const rawMenus = ref<any[]>([])

// 1. Tải cây menu từ Backend khi mở trang
onMounted(async () => {
  const token = localStorage.getItem('access_token')
  const res = await fetch('/api/v1/menus/user-menus', {
    headers: { Authorization: `Bearer ${token}` }
  })
  const json = await res.json()
  rawMenus.value = json.data || []
})

// 2. Hàm động chuyển chuỗi tên Icon (vd: "ProjectOutlined") thành Antd Icon Component
const renderIcon = (iconName?: string) => {
  if (!iconName || !(iconName in AntdIcons)) return null
  return () => h((AntdIcons as any)[iconName])
}

// 3. Đệ quy chuyển đổi icon chuỗi sang VNode cho a-menu
const transformMenus = (items: any[]): any[] => {
  return items.map(item => ({
    key: item.key,
    label: item.label,
    icon: renderIcon(item.icon),
    children: item.children && item.children.length > 0 ? transformMenus(item.children) : undefined,
    path: item.path
  }))
}

const menuItems = computed(() => transformMenus(rawMenus.value))

// 4. Bấm vào Menu -> Điều hướng trang
const handleMenuClick = ({ key }: { key: string }) => {
  if (key && key.startsWith('/')) {
    router.push(key)
  }
}
</script>
```

---

## 3. Màn Hình Cấu Hình Menu Cho Quản Trị Viên (SA)

Khi SA tạo hoặc sửa Menu, trường **"Quyền yêu cầu"** được gắn thẳng vào thẻ `<a-select>` của Ant Design.

### 3.1 Dropdown chọn Quyền yêu cầu với `<a-select>`

```vue
<!-- Modal Thêm/Sửa Menu -->
<template>
  <a-form :model="form" layout="vertical">
    <a-form-item label="Tên Menu hiển thị" required>
      <a-input v-model:value="form.title" placeholder="vd: Việc của tôi" />
    </a-form-item>

    <a-form-item label="Mã Menu (Code)" required>
      <a-input v-model:value="form.code" placeholder="vd: tasks_my" />
    </a-form-item>

    <a-form-item label="Đường dẫn (Path)">
      <a-input v-model:value="form.path" placeholder="vd: /tasks/my-tasks" />
    </a-form-item>

    <a-form-item label="Tên Icon (Ant Design Icons)">
      <a-input v-model:value="form.icon" placeholder="vd: CheckSquareOutlined, ProjectOutlined" />
    </a-form-item>

    <!-- Ô CHỌN QUYỀN YÊU CẦU: Nhận options trực tiếp từ Backend -->
    <a-form-item label="Quyền yêu cầu để hiển thị">
      <a-select
        v-model:value="form.required_permission"
        show-search
        allow-clear
        placeholder="Chọn quyền yêu cầu (bỏ trống nếu ai cũng xem được)"
        :options="permissionOptions"
      />
    </a-form-item>
  </a-form>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

const form = ref({
  title: '',
  code: '',
  path: '',
  icon: 'MenuOutlined',
  required_permission: undefined
})

const permissionOptions = ref([])

onMounted(async () => {
  const token = localStorage.getItem('access_token')
  const res = await fetch('/api/v1/permissions/options', {
    headers: { Authorization: `Bearer ${token}` }
  })
  const json = await res.json()
  // json.data có sẵn dạng: [{ value: "tasks:view", label: "Xem danh sách công việc (tasks:view)", module: "..." }]
  permissionOptions.value = (json.data || []).map((p: any) => ({
    value: p.value,
    label: p.label
  }))
})
</script>
```

---

## 4. Màn Hình Ma Trận Phân Quyền (Role - Permission Matrix)

Cho phép SA xem và tích chọn cấp quyền linh hoạt cho từng vai trò thông qua Ant Design `<a-table>` hoặc `<a-checkbox-group>`.

### 4.1 Quy trình thực hiện
1. **Lấy danh sách quyền hệ thống**: Gọi `GET /api/v1/permissions` (được nhóm theo `module`).
2. **Lấy danh sách quyền hiện tại của Role**: Gọi `GET /api/v1/roles/:roleId/permissions`.
3. **Khi SA bấm Lưu**: Gửi mảng các mã quyền được chọn qua `PUT /api/v1/roles/:roleId/permissions`:
   ```json
   {
     "permission_codes": [
       "tasks:view",
       "tasks:create",
       "tasks:review",
       "org:view_tree"
     ]
   }
   ```

---

## 5. Kiểm Soát Nút Bấm & Chặn Chuyển Trang (Security & UX)

### 5.1 Lưu tập quyền vào Pinia Store sau khi Đăng nhập

API `GET /api/v1/auth/me` hoặc `POST /api/v1/auth/login` trả về danh sách `permissions` của tài khoản:

```json
{
  "data": {
    "user": {
      "username": "truongphong",
      "roles": ["department_head"],
      "permissions": [
        "tasks:view",
        "tasks:view_dept",
        "tasks:create",
        "tasks:review",
        "tasks:update_progress",
        "org:view_tree"
      ]
    }
  }
}
```

Tạo hàm tiện ích `hasPermission`:
```typescript
// stores/auth.ts
export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as any,
    permissions: [] as string[]
  }),
  actions: {
    hasPermission(code: string): boolean {
      if (this.user?.roles?.includes('system_admin')) return true
      return this.permissions.includes(code)
    }
  }
})
```

### 5.2 Custom Directive ẩn/hiện Nút bấm (`v-can`)

```typescript
// plugins/permission.ts
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.directive('can', {
    mounted(el, binding) {
      const authStore = useAuthStore()
      const requiredPerm = binding.value
      if (!authStore.hasPermission(requiredPerm)) {
        el.parentNode?.removeChild(el) // Gỡ bỏ nút bấm khỏi DOM nếu không có quyền
      }
    }
  })
})
```

**Sử dụng trên giao diện**:
```vue
<!-- Chỉ ai có quyền tasks:create mới thấy nút Giao việc -->
<a-button type="primary" v-can="'tasks:create'" @click="openModal">
  <PlusOutlined /> Giao việc mới
</a-button>

<!-- Chỉ ai có quyền tasks:review mới thấy nút Nghiệm thu -->
<a-button type="success" v-can="'tasks:review'" @click="handleReview">
  <CheckOutlined /> Nghiệm thu
</a-button>
```

### 5.3 Chặn gõ trực tiếp URL trên trình duyệt (Route Guard)

```typescript
// middleware/auth.global.ts
export default defineNuxtRouteMiddleware((to) => {
  const authStore = useAuthStore()
  const requiredPerm = to.meta.requiredPermission as string | undefined

  if (requiredPerm && !authStore.hasPermission(requiredPerm)) {
    return navigateTo('/403') // Chuyển hướng tới trang Không có quyền truy cập
  }
})
```

---

## 6. Tổng Kết

Với kiến trúc trên:
1. **Frontend nhẹ nhàng**: Menu hoàn toàn động, gắn thẳng vào `<a-menu :items="items">` của Ant Design Vue.
2. **Quản trị tiện lợi**: SA toàn quyền quản lý menu, gán quyền bằng checkbox trên màn hình trực quan.
3. **Bảo mật tuyệt đối**: Dù người dùng cố tình can thiệp mã nguồn Frontend, Backend vẫn chặn các request trái phép với mã lỗi `403 FORBIDDEN`.

