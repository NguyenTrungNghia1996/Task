# TÀI LIỆU HƯỚNG DẪN TÍCH HỢP TOÀN DIỆN API BACKEND CHO FRONTEND

> **Dành cho**: Đội ngũ phát triển Frontend (React, Vue, Ant Design, Tailwind CSS, Mobile App, v.v.)  
> **Backend Version**: v1.1 (Golang Fiber + PostgreSQL / SQLite + MinIO / Local Storage)  
> **Swagger UI Trực quan**: `http://localhost:8080/swagger/index.html`  
> **OpenAPI 3.0 Specs**: `http://localhost:8080/openapi.json`

---

## MỤC LỤC
1. [Quy chuẩn Chung & Cấu trúc Dữ liệu](#1-quy-chuẩn-chung--cấu-trúc-dữ-liệu)
2. [Tài khoản Mẫu để Kiểm thử (Seeder)](#2-tài-khoản-mẫu-để-kiểm-thử-seeder)
3. [Cấu hình Axios Client & Tự động Refresh Token](#3-cấu-hình-axios-client--tự-động-refresh-token)
4. [Module 1: Xác thực & Phiên làm việc (/auth)](#4-module-1-xác-thực--phiên-làm-việc-auth)
5. [Module 2: Menu Động & Phân quyền (/menus, /rbac)](#5-module-2-menu-động--phân-quyền-menus-rbac)
6. [Module 3: Cơ cấu Tổ chức & Phòng ban (/departments)](#6-module-3-cơ-cấu-tổ-chức--phòng-ban-departments)
7. [Module 4: Quản lý Người dùng (/users)](#7-module-4-quản-lý-người-dùng-users)
8. [Module 5: Quản lý Công việc - Module Trọng tâm (/tasks)](#8-module-5-quản-lý-công-việc---module-trọng-tâm-tasks)
   - [8.1 Vòng đời Trạng thái (State Machine)](#81-vòng-đời-trạng-thái-state-machine)
   - [8.2 Tạo Công việc & Bản nháp (Draft)](#82-tạo-công-việc--bản-nháp-draft)
   - [8.3 Phát hành Bản nháp & Xóa An toàn](#83-phát-hành-bản-nháp--xóa-an-toàn)
   - [8.4 Hàng đợi Nhận việc & Tự nhận việc (Claim)](#84-hàng-đợi-nhận-việc--tự-nhận-việc-claim)
   - [8.5 Báo Vướng mắc (Block) & Gỡ Vướng mắc (Unblock)](#85-báo-vướng-mắc-block--gỡ-vướng-mắc-unblock)
   - [8.6 Cập nhật Tiến độ & Chờ Nghiệm thu (Submit)](#86-cập-nhật-tiến-độ--chờ-nghiệm-thu-submit)
   - [8.7 Nghiệm thu & Phê duyệt (Review)](#87-nghiệm-thu--phê-duyệt-review)
   - [8.8 Kéo thả Kanban & Đổi Nhánh Cây (Move Task)](#88-kéo-thả-kanban--đổi-nhánh-cây-move-task)
   - [8.9 Cây Nhiệm vụ Đa cấp Đệ quy (Task Tree)](#89-cây-nhiệm-vụ-đa-cấp-đệ-quy-task-tree)
   - [8.10 Quản lý Tệp đính kèm (Attachments: Upload & Download)](#810-quản-lý-tệp-đính-kèm-attachments-upload--download)
   - [8.11 Bình luận & Dòng sự kiện (Comments & Audit Log)](#811-bình-luận--dòng-sự-kiện-comments--audit-log)
9. [Module 6: Dashboard & Báo cáo Số liệu (/dashboard, /reports)](#9-module-6-dashboard--báo-cáo-số-liệu-dashboard-reports)
   - [9.1 KPI Tổng quan & Việc khẩn của tôi](#91-kpi-tổng-quan--việc-khẩn-của-tôi)
   - [9.2 Xuất Báo cáo Excel (.xlsx)](#92-xuất-báo-cáo-excel-xlsx)
10. [Module 7: Thông báo Thời gian thực (/notifications)](#10-module-7-thông-báo-thời-gian-thực-notifications)
    - [10.1 Quản lý Thông báo (List, Unread count, Mark Read)](#101-quản-lý-thông-báo-list-unread-count-mark-read)
    - [10.2 Server-Sent Events (SSE) & WebSocket](#102-server-sent-events-sse--websocket)
11. [Kinh nghiệm Xử lý Xung đột Phiên bản (Optimistic Locking)](#11-kinh-nghiệm-xử-lý-xung-đột-phiên-bản-optimistic-locking)

---

## 1. Quy chuẩn Chung & Cấu trúc Dữ liệu

### 1.1 Base URL
- **Môi trường phát triển**: `http://localhost:8080/api/v1`
- Hầu hết các API đều yêu cầu Header:
  ```http
  Authorization: Bearer <access_token>
  Content-Type: application/json
  ```
- Riêng kết nối **SSE** (`/notifications/stream`) và **WebSocket** (`/notifications/ws`), do trình duyệt không cho phép gắn Header tùy biến nên hỗ trợ truyền qua Query:
  ```
  http://localhost:8080/api/v1/notifications/stream?token=<access_token>
  ws://localhost:8080/api/v1/notifications/ws?token=<access_token>
  ```

### 1.2 Cấu trúc Phản hồi Thành công (Success Response)
```json
{
  "data": { ... }, // Đối tượng hoặc danh sách
  "meta": {        // Chỉ xuất hiện khi có phân trang
    "page": 1,
    "page_size": 20,
    "total": 48,
    "total_pages": 3
  }
}
```

### 1.3 Cấu trúc Phản hồi Lỗi (Error Response)
```json
{
  "error": {
    "code": "TASK_VERSION_CONFLICT",
    "message": "Dữ liệu công việc đã bị thay đổi bởi người khác",
    "details": null,
    "request_id": "c40e49c2-bb23-4245-9bfb-aeac58dcdbfa"
  }
}
```

---

## 2. Tài khoản Mẫu để Kiểm thử (Seeder)

Tất cả tài khoản mẫu có mật khẩu mặc định là: **`password123`**

| Tên đăng nhập | Họ và tên | Vai trò (Role) | Phòng ban phụ trách | Mục đích kiểm thử |
| :--- | :--- | :--- | :--- | :--- |
| `admin` | Quản trị Hệ thống | `system_admin` | Toàn hệ thống | Quản trị Menu, RBAC, phân quyền tài khoản |
| `commander` | Nguyễn Văn Chỉ Huy | `commander` | Toàn đơn vị | Giao việc cấp cao, phê duyệt nghiệm thu, xem Dashboard |
| `truongphong` | Trần Văn Trưởng Phòng | `department_head` | Phòng Tham Mưu | Giao việc con, phê duyệt tiến độ, quản lý phòng |
| `phophong` | Phạm Thị Phó Phòng | `department_head` | Phòng Kế Hoạch | Quản lý kế hoạch, nhận việc phòng |
| `nhanvien` | Lê Văn Nhân Viên | `employee` | Ban Thông Tin | Thực hiện việc, báo cáo tiến độ, tải tệp, báo vướng mắc |
| `tovien` | Hoàng Văn Tổ Viên | `team_member` | Tổ Kỹ Thuật | Nhận việc từ hàng đợi, nộp nghiệm thu |

---

## 3. Cấu hình Axios Client & Tự động Refresh Token

Tạo tệp `src/services/apiClient.ts` trong dự án Frontend:

```typescript
import axios from 'axios';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request Interceptor: Tự động gắn Bearer Token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response Interceptor: Tự động bắt lỗi 401 và làm mới Access Token
let isRefreshing = false;
let failedQueue: Array<{ resolve: (token: string) => void; reject: (err: any) => void }> = [];

const processQueue = (error: any, token: string | null = null) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token!);
    }
  });
  failedQueue = [];
};

apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 401 && !originalRequest._retry) {
      if (originalRequest.url?.includes('/auth/login') || originalRequest.url?.includes('/auth/refresh')) {
        return Promise.reject(error);
      }

      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        })
          .then((token) => {
            originalRequest.headers.Authorization = `Bearer ${token}`;
            return apiClient(originalRequest);
          })
          .catch((err) => Promise.reject(err));
      }

      originalRequest._retry = true;
      isRefreshing = true;

      const refreshToken = localStorage.getItem('refresh_token');
      if (!refreshToken) {
        localStorage.clear();
        window.location.href = '/login';
        return Promise.reject(error);
      }

      try {
        const res = await axios.post(`${API_BASE_URL}/auth/refresh`, {
          refresh_token: refreshToken,
        });
        const { access_token, refresh_token: newRefreshToken } = res.data.data;
        localStorage.setItem('access_token', access_token);
        localStorage.setItem('refresh_token', newRefreshToken);

        apiClient.defaults.headers.common.Authorization = `Bearer ${access_token}`;
        processQueue(null, access_token);

        return apiClient(originalRequest);
      } catch (refreshErr) {
        processQueue(refreshErr, null);
        localStorage.clear();
        window.location.href = '/login';
        return Promise.reject(refreshErr);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  }
);
```

---

## 4. Module 1: Xác thực & Phiên làm việc (/auth)

### 4.1 Đăng nhập
- **Endpoint**: `POST /auth/login`
- **Body**:
  ```json
  {
    "username": "admin",
    "password": "password123"
  }
  ```
- **Response `200 OK`**:
  ```json
  {
    "data": {
      "access_token": "eyJhbGciOi...",
      "refresh_token": "eyJhbGciOi...",
      "token_type": "Bearer",
      "expires_in": 900,
      "user": {
        "id": "e44d348a-...",
        "username": "admin",
        "full_name": "Quản trị Hệ thống",
        "email": "admin@example.com",
        "roles": ["system_admin"],
        "organization_id": "060d40e9-..."
      }
    }
  }
  ```

### 4.2 Lấy thông tin tài khoản hiện tại (Me)
- **Endpoint**: `GET /auth/me`
- **Response `200 OK`**: Trả về `user` chi tiết, danh sách vai trò (`roles`) và quyền hạn cụ thể (`permissions`).

### 4.3 Đăng xuất
- **Endpoint**: `POST /auth/logout`
- **Body**: `{ "refresh_token": "..." }`

---

## 5. Module 2: Menu Động & Phân quyền (/menus, /rbac)

### 5.1 Lấy Cây Menu Động Ant Design cho Sidebar
- **Endpoint**: `GET /menus/my-menus`
- **Mô tả**: Tự động lọc menu theo các vai trò của người dùng đang đăng nhập. Dữ liệu trả về đúng chuẩn giao diện `<Menu items={menuTree} />` của Ant Design.
- **Response `200 OK`**:
  ```json
  {
    "data": [
      {
        "key": "dashboard",
        "label": "Bàn làm việc",
        "icon": "DashboardOutlined",
        "path": "/dashboard",
        "sort_order": 1
      },
      {
        "key": "tasks",
        "label": "Quản lý Công việc",
        "icon": "ProjectOutlined",
        "path": "/tasks",
        "sort_order": 2,
        "children": [
          { "key": "tasks-kanban", "label": "Bảng Kanban", "path": "/tasks/kanban", "sort_order": 1 },
          { "key": "tasks-tree", "label": "Cây nhiệm vụ", "path": "/tasks/tree", "sort_order": 2 },
          { "key": "tasks-pool", "label": "Hàng đợi nhận việc", "path": "/tasks/pool", "sort_order": 3 }
        ]
      }
    ]
  }
  ```

### 5.2 Quản lý Vai trò & Ma trận Quyền
- `GET /rbac/roles`: Danh sách vai trò.
- `GET /rbac/permissions`: Danh sách tất cả quyền hạn phân theo nhóm module (`tasks`, `users`, `departments`, `reports`, `menus`, `rbac`).
- `GET /rbac/roles/:roleId/permissions`: Lấy danh sách quyền đã gán cho vai trò.
- `PUT /rbac/roles/:roleId/permissions`: Cập nhật quyền cho vai trò:
  ```json
  {
    "permission_ids": ["uuid-1", "uuid-2"]
  }
  ```

---

## 6. Module 3: Cơ cấu Tổ chức & Phòng ban (/departments)

### 6.1 Cây phân cấp Phòng ban
- **Endpoint**: `GET /departments/tree`
- **Response**: Trả về cấu trúc cây cha - con nhiều cấp, có sẵn thông tin `leader` (chỉ huy/trưởng phòng) và số lượng `member_count`. Rất phù hợp hiển thị `<Tree />` hoặc `<TreeSelect />` của Ant Design.

### 6.2 Danh sách Phòng ban dạng phẳng (Dropdown Options)
- **Endpoint**: `GET /departments/options`
- **Response**: Trả về mảng `[{ "id": "...", "code": "PTM", "name": "Phòng Tham Mưu" }]`.

### 6.3 Quản lý Thành viên Phòng ban
- `GET /departments/:departmentId/members`: Danh sách thành viên trong phòng.
- `POST /departments/:departmentId/members`: Thêm thành viên vào phòng:
  ```json
  {
    "user_id": "uuid-cua-nhan-vien",
    "member_role": "member", // "leader" hoặc "member"
    "is_primary": true
  }
  ```
- `DELETE /departments/:departmentId/members/:memberId`: Xóa thành viên khỏi phòng ban.

---

## 7. Module 4: Quản lý Người dùng (/users)

- `GET /users?page=1&page_size=20&q=tu_khoa&department_id=uuid&is_active=true`: Danh sách tài khoản có phân trang và tìm kiếm.
- `GET /users/:userId`: Chi tiết người dùng kèm danh sách vai trò và phòng ban.
- `POST /users`: Tạo người dùng mới:
  ```json
  {
    "username": "hoangvan",
    "password": "password123",
    "full_name": "Hoàng Văn A",
    "email": "hoangvana@example.com",
    "department_id": "uuid-phong-ban",
    "role_codes": ["employee"]
  }
  ```
- `PUT /users/:userId/roles`: Gán danh sách vai trò cho user: `{"role_codes": ["department_head"]}`.
- `POST /users/:userId/reset-password`: Đặt lại mật khẩu: `{"new_password": "newpassword123"}`.

---

## 8. Module 5: Quản lý Công việc - Module Trọng tâm (/tasks)

### 8.1 Vòng đời Trạng thái (State Machine)

```
                    ┌──────────────┐
                    │    draft     │ (Bản nháp - Chỉ xóa được khi ở đây)
                    └──────┬───────┘
                           │ (Publish)
             ┌─────────────┴─────────────┐
             ▼                           ▼
     ┌──────────────┐             ┌──────────────┐
     │  unassigned  │ (Chờ nhận)  │   assigned   │ (Đã giao việc)
     └───────┬──────┘             └──────┬───────┘
             │ (Claim - Tự nhận)         │
             └─────────────┬─────────────┘
                           ▼
                  ┌─────────────────┐
     ┌───────────►│   in_progress   │◄───────────┐
     │            └───────┬──▲──────┘            │
     │                    │  │                   │
 (Unblock)       (Block)  │  │ (Revision Req) (Reopen)
     │                    ▼  │                   │
┌────┴────────┐   ┌───────┴──┴──────┐            │
│   blocked   │   │ revision_req    │            │
└─────────────┘   └─────────────────┘            │
                          │                      │
                  (Tiến độ 100% hoặc Submit)     │
                          ▼                      │
                  ┌─────────────────┐            │
                  │    submitted    │            │
                  └───────┬─────────┘            │
                          │ (Review: Approve)    │
                          ▼                      │
                  ┌─────────────────┐            │
                  │    completed    ├────────────┘
                  └─────────────────┘
```

---

### 8.2 Tạo Công việc & Bản nháp (Draft)
- **Endpoint**: `POST /tasks`
- **Body**:
  ```json
  {
    "title": "Kế hoạch diễn tập quý 4",
    "description": "Nội dung chỉ đạo diễn tập phòng không",
    "priority": 1,                     // 1: Khẩn, 2: Bình thường, 3: Thấp
    "start_at": "2026-11-05T08:00:00Z",
    "due_at": "2026-11-20T17:00:00Z",
    "approval_required": true,
    "sort_order": 1,
    "is_draft": true,                  // TRUE: Lưu bản nháp (không thông báo)
    "initial_assignment": {            // Có thể NULL nếu để trong hàng đợi chờ nhận việc!
      "type": "user",                  // "user" hoặc "department"
      "id": "uuid-nguoi-nhan",
      "responsible_user_id": null
    }
  }
  ```

---

### 8.3 Phát hành Bản nháp & Xóa An toàn
* **Phát hành công việc**: `POST /tasks/:taskId/publish`
  - Chuyển từ `draft` sang `assigned` (nếu đã gán người) hoặc `unassigned` (nếu chưa gán).
  - Tự động kích hoạt thông báo cho người được phân công.
* **Xóa công việc**: `DELETE /tasks/:taskId`
  - **Lưu ý quan trọng cho FE**: Chỉ gọi được khi `status === 'draft'`. Nếu task đã giao, backend sẽ trả về lỗi `400 Bad Request` (`CANNOT_DELETE_DISPATCHED_TASK`). Lúc này FE phải ẩn nút "Xóa" và hiển thị nút "Hủy công việc" (`POST /tasks/:taskId/cancellations`).

---

### 8.4 Hàng đợi Nhận việc & Tự nhận việc (Claim)
- **Lấy danh sách việc chưa gán người**:
  `GET /tasks?is_assigned=false&status=unassigned`
- **Nhân viên bấm "Nhận việc"**:
  - **Endpoint**: `POST /tasks/:taskId/claim`
  - Người dùng đăng nhập sẽ tự động trở thành người phụ trách chính (`is_primary = true`), task chuyển sang trạng thái `assigned`.

---

### 8.5 Báo Vướng mắc (Block) & Gỡ Vướng mắc (Unblock)
* **Báo vướng mắc**: `POST /tasks/:taskId/block`
  ```json
  {
    "reason": "Thiếu linh kiện máy chủ chưa kịp nhập khẩu"
  }
  ```
  *(Trạng thái task chuyển thành `blocked`, trường `blocked_reason` được cập nhật)*.
* **Gỡ vướng mắc**: `POST /tasks/:taskId/unblock`
  ```json
  {
    "note": "Linh kiện đã về kho và kiểm thử đạt yêu cầu"
  }
  ```
  *(Trạng thái task quay về `in_progress`, xóa `blocked_reason`)*.

---

### 8.6 Cập nhật Tiến độ & Chờ Nghiệm thu (Submit)
* **Cập nhật tiến độ**: `POST /tasks/:taskId/updates`
  ```json
  {
    "progress": 80,
    "content": "Đã hoàn thành cấu hình mạng và cài đặt phần mềm",
    "attachment_ids": ["uuid-tep-dinh-kem-1"]
  }
  ```
  > **Quy tắc tự động**: Nếu `progress === 100` và task có `approval_required: true`, Backend sẽ **tự động chuyển trạng thái task sang `submitted`** (Chờ nghiệm thu)!
* **Chủ động nộp nghiệm thu**: `POST /tasks/:taskId/submit`

---

### 8.7 Nghiệm thu & Phê duyệt (Review)
- **Endpoint**: `POST /tasks/:taskId/reviews`
- **Body**:
  ```json
  {
    "update_id": "uuid-lan-cap-nhat-gan-nhat",
    "decision": "approved", // "approved" hoặc "revision_required"
    "comment": "Chất lượng hoàn thiện tốt, nghiệm thu bàn giao."
  }
  ```
  - Nếu `approved`: Task chuyển sang `completed`.
  - Nếu `revision_required`: Task chuyển sang `revision_required` yêu cầu bổ sung.

---

### 8.8 Kéo thả Kanban & Đổi Nhánh Cây (Move Task)
- **Endpoint**: `PATCH /tasks/:taskId/move`
- **Hỗ trợ đầy đủ cho UI Drag & Drop**:
  ```json
  {
    "new_status": "in_progress", // Kéo sang cột khác (assigned, in_progress, blocked, submitted, completed)
    "sort_order": 3,             // Đổi vị trí thứ tự hiển thị trong cột
    "parent_id": "uuid-cha-moi", // NULL nếu đưa ra làm việc gốc, hoặc UUID việc cha mới
    "version": 2                 // BẮT BUỘC: Version hiện tại của task (Optimistic Locking)
  }
  ```
  > **Bảo vệ chống vòng lặp**: Backend tự động kiểm tra chống kéo một việc cha vào làm con của chính nó hoặc con của con cháu nó (trả về lỗi `CYCLIC_DEPENDENCY`).

---

### 8.9 Cây Nhiệm vụ Đa cấp Đệ quy (Task Tree)
- **Endpoint**: `GET /tasks/:taskId/tree`
- **Mô tả**: Tải toàn bộ cây phân cấp từ việc gốc xuống tất cả các đời con cháu đệ quy vô hạn cấp. Mỗi nút đều preloaded thông tin người được giao (`assignments`). Rất dễ dàng render bảng `<Table />` dạng cây phân cấp của Ant Design.

---

### 8.10 Quản lý Tệp đính kèm (Attachments: Upload & Download)

Hệ thống hỗ trợ 2 cơ chế tùy theo cấu hình backend:

#### Cách 1: Tải lên Trực tiếp qua Golang (Multipart Upload - Phổ biến nhất)
- **Endpoint**: `POST /tasks/:taskId/attachments/upload`
- **Kiểu dữ liệu**: `multipart/form-data`
- **Trường file**: `file` (Binary)
- **Code mẫu Axios Frontend**:
  ```typescript
  const formData = new FormData();
  formData.append('file', fileObject);

  const res = await apiClient.post(`/tasks/${taskId}/attachments/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  const attachment = res.data.data; // Trả về TaskAttachment có ID để gắn vào update
  ```

#### Cách 2: Tải lên 2 bước với MinIO / S3 Presigned URL
1. Bước 1: `POST /tasks/:taskId/attachments/presign`
   - Body: `{"file_name": "tailieu.pdf", "content_type": "application/pdf", "size_bytes": 1048576}`
   - Nhận về: `upload_url` và `attachment_id`.
2. Bước 2: Frontend dùng `axios.put(upload_url, fileObject, { headers: { 'Content-Type': 'application/pdf' } })` đẩy thẳng lên S3/MinIO.
3. Bước 3: `POST /tasks/:taskId/attachments/confirm` với `{"attachment_id": "..."}` để backend kích hoạt tệp.

#### Tải xuống & Xem tệp đính kèm
- **Lấy danh sách tệp của task**: `GET /tasks/:taskId/attachments`
- **Tải file**: `GET /tasks/:taskId/attachments/:attachmentId/download`
  ```typescript
  export const downloadAttachment = async (taskId: string, attachmentId: string, fileName: string) => {
    const response = await apiClient.get(`/tasks/${taskId}/attachments/${attachmentId}/download`, {
      responseType: 'blob',
    });
    const url = window.URL.createObjectURL(new Blob([response.data]));
    const link = document.createElement('a');
    link.href = url;
    link.setAttribute('download', fileName);
    document.body.appendChild(link);
    link.click();
    link.remove();
  };
  ```
- **Xóa tệp**: `DELETE /tasks/:taskId/attachments/:attachmentId`

---

### 8.11 Bình luận & Dòng sự kiện (Comments & Audit Log)
- `GET /tasks/:taskId/comments`: Danh sách bình luận trao đổi.
- `POST /tasks/:taskId/comments`: Gửi bình luận `{"content": "Đồng chí gửi lại sơ đồ mạng"}`.
- `GET /tasks/:taskId/events`: Lấy toàn bộ dòng thời gian lịch sử thao tác (Audit Log: tạo việc, sửa hạn, nộp tiến độ, đổi trạng thái, ai làm vào giờ nào).

---

## 9. Module 6: Dashboard & Báo cáo Số liệu (/dashboard, /reports)

### 9.1 KPI Tổng quan & Việc khẩn của tôi
- **Endpoint**: `GET /dashboard/summary`
  - Trả về: `total_tasks`, `in_progress_tasks`, `completed_tasks`, `blocked_tasks`, `overdue_tasks`, `avg_progress`, `completion_rate`.
- **Endpoint**: `GET /dashboard/my-tasks?limit=10`
  - Lấy các công việc cấp bách được phân công cho user hiện tại (ưu tiên việc quá hạn và ưu tiên 1 lên đầu).

### 9.2 Xuất Báo cáo Excel (.xlsx)

Frontend chỉ cần gọi và tải trực tiếp dạng Blob:

```typescript
export const exportReportExcel = async (type: 'tasks' | 'departments', filterParams = {}) => {
  const endpoint = type === 'tasks' ? '/reports/tasks/export' : '/reports/departments/export';
  const defaultName = type === 'tasks' ? 'Bao_cao_cong_viec.xlsx' : 'Bao_cao_hieu_suat_phong_ban.xlsx';

  const response = await apiClient.get(endpoint, {
    params: filterParams,
    responseType: 'blob',
  });

  const blob = new Blob([response.data], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  });
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = defaultName;
  document.body.appendChild(a);
  a.click();
  a.remove();
};
```

---

## 10. Module 7: Thông báo Thời gian thực (/notifications)

### 10.1 Quản lý Thông báo
- `GET /notifications?page=1&page_size=15&unread_only=true`: Danh sách thông báo.
- `GET /notifications/unread-count`: Đếm nhanh số lượng chưa đọc (gắn badge trên icon chuông).
- `PATCH /notifications/:notificationId`: Đánh dấu 1 thông báo đã đọc.
- `POST /notifications/read-all`: Đánh dấu tất cả đã đọc.

### 10.2 Server-Sent Events (SSE) & WebSocket

Frontend có thể chọn 1 trong 2 giải pháp kết nối đẩy tức thời:

#### Giải pháp A: Server-Sent Events (SSE - Đơn giản nhất)
```typescript
import { useEffect, useState } from 'react';

export const useNotificationSSE = () => {
  const [latestNotification, setLatestNotification] = useState<any>(null);

  useEffect(() => {
    const token = localStorage.getItem('access_token');
    if (!token) return;

    const eventSource = new EventSource(`http://localhost:8080/api/v1/notifications/stream?token=${token}`);

    eventSource.addEventListener('notification', (e) => {
      const data = JSON.parse(e.data);
      setLatestNotification(data);
      // Hiển thị Ant Design Notification Popup
      // notification.info({ message: data.title, description: data.body });
    });

    eventSource.onerror = () => {
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, []);

  return latestNotification;
};
```

#### Giải pháp B: WebSocket (Hai chiều)
```typescript
export const connectNotificationWS = (onMessage: (notif: any) => void) => {
  const token = localStorage.getItem('access_token');
  const ws = new WebSocket(`ws://localhost:8080/api/v1/notifications/ws?token=${token}`);

  ws.onmessage = (event) => {
    try {
      const notif = JSON.parse(event.data);
      onMessage(notif);
    } catch (err) {
      console.error(err);
    }
  };

  return ws;
};
```

---

## 11. Kinh nghiệm Xử lý Xung đột Phiên bản (Optimistic Locking)

Để đảm bảo nhiều người cùng thao tác một công việc (kéo thẻ, sửa hạn) không bị ghi đè mất dữ liệu:
1. Mỗi đối tượng `Task` trả về từ Backend luôn có trường `"version": number`.
2. Khi gọi `PATCH /tasks/:taskId` hoặc `PATCH /tasks/:taskId/move`, FE **bắt buộc gửi kèm `version` hiện tại**.
3. Nếu người khác đã cập nhật trước, Backend sẽ trả về mã `409 Conflict`:
   ```json
   {
     "error": {
       "code": "TASK_VERSION_CONFLICT",
       "message": "Dữ liệu công việc đã bị thay đổi bởi người khác, vui lòng tải lại trước khi lưu"
     }
   }
   ```
4. **Cách xử lý tại FE**:
   - Khi nhận mã lỗi `409 Conflict`, hiển thị thông báo nhẹ: *"Dữ liệu công việc vừa có thay đổi từ người khác. Đang tự động làm mới..."*.
   - Gọi lại `GET /tasks/:taskId` hoặc `GET /tasks` để cập nhật dữ liệu và `version` mới nhất lên giao diện.

---
*Chúc các bạn xây dựng giao diện người dùng mượt mà, chuyên nghiệp và hiệu quả!*
