# Đặc tả kỹ thuật v1.1 --- Ứng dụng quản lý công việc nhiều cấp

-   **Backend:** Golang (Fiber v2/v3)
-   **ORM & Query:** GORM + Custom recursive CTE queries (PostgreSQL)
-   **Database:** PostgreSQL 15+
-   **Storage:** MinIO / AWS S3 (Presigned URLs)
-   **Frontend:** Nuxt 4 (SPA mode `ssr: false`) + Vue 3 + TypeScript + Ant Design Vue + Pinia
-   **Kiến trúc:** Modular Monolith, REST API `/api/v1`
-   **Trạng thái tài liệu:** Phiên bản v1.1 (Đã hoàn thiện sau thẩm định kiến trúc: chuẩn hóa Optimistic Locking với `version`, phân định rõ Xóa mềm (`draft`) vs Hủy nghiệp vụ (`cancelled`), loại trừ task hủy khỏi công thức tiến độ để tránh deadlock, chuẩn hóa luồng Upload 2 bước với Presigned URL, bổ sung đầy đủ Go struct GORM và DTO).

## 1. Mục tiêu và phạm vi MVP

Ứng dụng cho phép chỉ huy giao nhiệm vụ cho phòng ban; trưởng phòng chia
thành công việc con giao cho nhân viên; nhân viên phụ trách có thể tiếp
tục chia việc cho tổ viên theo quyền. Cấp trên theo dõi tiến độ xuyên
suốt cây công việc và nghiệm thu kết quả.

### Module

1.  Tổ chức: đơn vị, phòng ban, tổ, nhân sự, quan hệ quản lý.
2.  Xác thực và phân quyền.
3.  Quản lý nhiệm vụ và công việc con nhiều cấp.
4.  Giao việc cho phòng ban hoặc cá nhân.
5.  Tiến độ, báo cáo, vướng mắc và nghiệm thu.
6.  Bình luận, tài liệu và nhật ký hoạt động.
7.  Dashboard, thông báo và báo cáo.
8.  Quản trị, audit log và vận hành.

Ngoài phạm vi MVP: chữ ký số, tích hợp hệ thống văn bản điện tử, ứng
dụng mobile native và workflow designer.

## 2. Mô hình tổ chức và phân quyền

### 2.1 Vai trò đề xuất

  Mã vai trò          Phạm vi mặc định
  ------------------- -------------------------------------------
  `system_admin`      Quản trị hệ thống theo quyền được cấp
  `commander`         Công việc toàn đơn vị được quản lý
  `department_head`   Phòng ban được phụ trách
  `manager`           Nhóm/cấp dưới thuộc phạm vi quản lý
  `employee`          Công việc được giao hoặc chia sẻ
  `team_member`       Công việc được giao và thao tác thực hiện

Vai trò hệ thống phải tách khỏi chức vụ trong tổ chức. Chức danh trưởng
phòng không tự động cấp quyền quản trị hệ thống. Quyền cần kết hợp RBAC
với phạm vi dữ liệu (scope-based authorization).

### 2.2 Nguyên tắc phân quyền

-   Mọi truy vấn nghiệp vụ đều giới hạn theo đơn vị và phạm vi quyền.
-   Kiểm tra quyền ở Backend cho mọi endpoint; ẩn menu ở Frontend chỉ là
    UX.
-   Không tin `organization_id`, `created_by`, `reviewer_id`,
    `root_task_id` hoặc trạng thái do client tự gửi.
-   Người nhận công việc không mặc nhiên được giao tiếp cho đối tượng
    ngoài phạm vi quản lý.
-   Một nhiệm vụ giao cho phòng ban phải có một người chịu trách nhiệm
    chính.
-   Người nghiệm thu phải là người có thẩm quyền giao việc/duyệt việc
    hoặc cấp quản lý cao hơn theo quy tắc tổ chức.

## 3. Đặc tả nghiệp vụ

### 3.1 Quy trình

1.  Chỉ huy tạo nhiệm vụ và giao cho phòng ban.
2.  Trưởng phòng tiếp nhận, tạo công việc con và giao cho nhân viên.
3.  Nhân viên có quyền quản lý có thể tiếp tục tạo công việc con cho tổ
    viên.
4.  Người thực hiện cập nhật tiến độ, báo vướng mắc, gửi báo cáo và tài
    liệu.
5.  Cấp có thẩm quyền nghiệm thu hoặc yêu cầu bổ sung.
6.  Tiến độ được tổng hợp lên các công việc cha; chỉ huy theo dõi toàn
    cây.

### 3.2 Trạng thái

  Trạng thái            Ý nghĩa
  --------------------- ---------------------------
  `draft`               Bản nháp
  `assigned`            Đã giao, chờ thực hiện
  `in_progress`         Đang thực hiện
  `blocked`             Đang vướng mắc
  `submitted`           Đã gửi báo cáo hoàn thành
  `revision_required`   Cần bổ sung/chỉnh sửa
  `completed`           Đã nghiệm thu
  `cancelled`           Đã hủy theo thẩm quyền

Quá hạn nên được tính từ `due_at` và trạng thái, không nhất thiết là
trạng thái lưu riêng. Chuyển trạng thái phải qua nghiệp vụ API; không
cho phép PATCH trạng thái tùy ý.

### 3.3 Quy tắc công việc con và thời hạn

**Quyết định đã chốt: thời hạn công việc con bắt buộc nằm trong thời hạn
công việc cha.**

-   Nếu công việc cha có `start_at`, công việc con không được bắt đầu
    trước thời điểm đó.
-   Nếu công việc cha có `due_at`, công việc con phải có `due_at` và
    `due_at <= parent.due_at`.
-   Nếu cha không có hạn hoàn thành, chính sách mặc định có thể cho phép
    con có hạn riêng; cần cấu hình rõ ở cấp đơn vị.
-   Khi sửa hạn của công việc cha khiến một hoặc nhiều công việc con
    vượt hạn, Backend từ chối cập nhật với `409 Conflict` hoặc
    `422 Unprocessable Content` và trả danh sách công việc bị ảnh hưởng.
    Không tự động đổi hạn con âm thầm.
-   Nếu cần ngoại lệ, phải có hành động riêng do cấp có thẩm quyền thực
    hiện, nhập lý do và ghi audit log; ngoại lệ phải được thể hiện rõ
    trong dữ liệu nghiệp vụ.
-   Không được tạo công việc con dưới công việc đã `completed` hoặc
    `cancelled`, trừ khi công việc được mở lại hợp lệ.
-   Chặn chu trình trong cây: không thể đặt công việc tổ tiên làm con
    của hậu duệ.

### 3.4 Tiến độ

Tổng hợp tiến độ công việc cha theo trọng số của các công việc con trực tiếp:

`progress(parent) = round( sum(weight_i * progress_i) / sum(weight_i) )`

-   `progress_i` nằm trong khoảng 0--100.
-   Mặc định `weight = 1`; trọng số phải là số nguyên dương lớn hơn 0.
-   Công việc lá (không có con) cho phép người phụ trách cập nhật tiến độ trực tiếp qua báo cáo `task_updates`.
-   Công việc có con: tiến độ được hệ thống tự động tổng hợp từ các công việc con trực tiếp; không cho phép sửa đè trực tiếp. Tiến độ đạt 100% không đồng nhất với trạng thái nghiệm thu hoàn thành (`completed`).
-   **Chính sách công việc con bị hủy (`cancelled`):**
    -   Loại hoàn toàn các công việc con có trạng thái `cancelled` ra khỏi công thức tổng hợp tiến độ (loại cả tử số `weight_i * progress_i` lẫn mẫu số `sum(weight_i)`).
    -   Quy tắc này ngăn chặn triệt để hiện tượng "kẹt tiến độ" (Progress Deadlock), đảm bảo khi tất cả các công việc con hợp lệ còn lại hoàn thành 100% thì công việc cha vẫn đạt 100% để có thể nghiệm thu.
-   Nếu toàn bộ công việc con đều bị hủy hoặc không có công việc con nào có trọng số hợp lệ: tiến độ cha giữ nguyên giá trị trước đó hoặc chuyển về chế độ cho phép người phụ trách/quản lý cập nhật thủ công.

### 3.5 Báo cáo và nghiệm thu

Mỗi lần submit tạo một bản ghi `task_updates`, không ghi đè lịch sử cũ.
Bản ghi lưu người gửi, thời điểm, tiến độ, nội dung và các tệp đính kèm.

`POST /tasks/{id}/reviews` hoặc endpoint REST tương đương tạo kết quả
đánh giá `approved` hoặc `revision_required`. Chỉ người có thẩm quyền
mới được nghiệm thu. Trước khi approve, kiểm tra báo cáo thuộc đúng
nhiệm vụ và các công việc con bắt buộc đã hoàn thành.

### 3.6 Mở lại công việc sau nghiệm thu

**Quyết định đã chốt: chỉ người nghiệm thu gần nhất hoặc cấp quản lý cao
hơn mới được mở lại công việc; bắt buộc nhập lý do và ghi audit log.**

-   Dùng hành động chuyên biệt `POST /api/v1/tasks/{id}/reopen`.
-   Request yêu cầu `reason` không rỗng, có giới hạn độ dài và được lưu
    nguyên văn.
-   Backend kiểm tra người gọi là reviewer gần nhất hoặc có cấp quyền
    cao hơn theo cây tổ chức/phạm vi nhiệm vụ.
-   Mở lại trong transaction: khóa công việc, kiểm tra trạng thái hiện
    tại, chuyển trạng thái phù hợp (thường `in_progress` hoặc
    `revision_required`), ghi `task_events` chứa lý do, actor, trạng
    thái trước/sau và thời điểm.
-   Không xóa hoặc sửa bản ghi nghiệm thu cũ.
-   Nếu công việc cha đã hoàn thành và cần mở lại để xử lý công việc
    con, phải xác định chính sách mở lại chuỗi tổ tiên liên quan; không
    tự động mở lại các cha mà không ghi sự kiện riêng.
-   Nếu người nghiệm thu gần nhất không còn hoạt động, cấp quản lý cao
    hơn được phép xử lý theo quyền được cấu hình.

### 3.7 Hủy và sửa công việc

-   **Phân định giữa Hủy nghiệp vụ và Xóa kỹ thuật (Soft Delete):**
    -   *Hủy nghiệp vụ (`status = 'cancelled'`):* Áp dụng cho các công việc đã phát sinh trạng thái (`assigned`, `in_progress`, ...). Bắt buộc nhập lý do, ghi `task_events` audit. Bản ghi **không** set `deleted_at` của GORM để dữ liệu vẫn hiển thị trên cây công việc, báo cáo lịch sử và audit log.
    -   *Xóa kỹ thuật (`DeletedAt`):* Chỉ áp dụng khi người tạo muốn xóa bỏ bản nháp (`draft`) chưa từng giao việc và chưa có phát sinh liên kết con/báo cáo.
-   Hủy công việc cha phải kiểm tra các công việc con đang hoạt động và áp dụng chính sách phù hợp (hủy cascade có audit log hoặc chặn hủy nếu con chưa giải quyết xong).
-   Mọi thay đổi người phụ trách, hạn hoàn thành, trạng thái và quyết định nghiệm thu phải lưu `task_events`.
-   Mọi thao tác sửa đổi thông tin quan trọng (`title`, `due_at`, `priority`) hoặc chuyển trạng thái đều yêu cầu kiểm tra trường `version` (Optimistic Locking) để tránh ghi đè dữ liệu đồng thời.

## 4. Kiến trúc kỹ thuật

-   **Backend:** Golang + Fiber (HTTP routing, middleware, request parsing hiệu năng cao).
-   **ORM & Query:** GORM cho CRUD và quan hệ cơ bản; sử dụng raw SQL / Recursive CTE (`WITH RECURSIVE`) cho truy vấn cây công việc và cây phòng ban nhằm đảm bảo tốc độ và kiểm soát memory.
-   **Database:** PostgreSQL 15+ (kiểu UUID, TIMESTAMPTZ, JSONB, CHECK constraints).
-   **Concurrency & Locking:** Kết hợp Optimistic Locking (`version` column) cho cập nhật metadata/trạng thái và Pessimistic Locking (`SELECT ... FOR UPDATE`) cho các luồng duyệt/nghiệm thu/mở lại và tính toán cây.
-   **REST JSON API:** Tiền tố `/api/v1`, chuẩn hóa định dạng request/response và error handling.
-   **Xác thực:** JWT Access Token ngắn hạn (15 phút); Refresh Token có hash SHA-256 lưu DB, thu hồi khi logout. Quản lý qua Authorization Bearer hoặc HttpOnly/Secure Cookie.
-   **Frontend:** Nuxt 4 cấu hình chạy chế độ **SPA (`ssr: false`)**, Pinia state management, Ant Design Vue, Axios/ofetch interceptor tự động làm mới token.
-   **Lưu trữ tệp:** MinIO hoặc S3-compatible; luồng upload 2 bước qua Presigned URL (Client lấy Presigned URL -> PUT trực tiếp lên MinIO/S3 -> Gọi API xác nhận lưu metadata). PostgreSQL chỉ lưu metadata và object key.
-   **Thông báo & Real-time:** REST API thông báo + hỗ trợ Server-Sent Events (SSE) `/api/v1/notifications/stream` cho cảnh báo giao việc/nghiệm thu tức thời; fallback qua polling định kỳ.
-   **Logging:** Structured logging (Zap/Slog), gắn `request_id` xuyên suốt context, tuyệt đối không log mật khẩu/token.
-   **Migration:** Công cụ migration có version (Golang-migrate/Goose); không dùng `AutoMigrate` làm chiến lược production duy nhất.
-   **Test:** Unit test, integration test với PostgreSQL, API authorization test.

## 5. ERD PostgreSQL

``` mermaid
erDiagram
    organizations ||--o{ departments : contains
    organizations ||--o{ users : owns
    departments ||--o{ departments : parent
    users ||--o{ department_members : joins
    departments ||--o{ department_members : includes
    users ||--o{ user_roles : has
    roles ||--o{ user_roles : grants

    organizations ||--o{ tasks : owns
    tasks o|--o{ tasks : parent_child
    tasks ||--o{ task_assignments : assignments
    users o|--o{ task_assignments : assigned_user
    departments o|--o{ task_assignments : assigned_department

    tasks ||--o{ task_updates : reports
    tasks ||--o{ task_reviews : reviews
    tasks ||--o{ task_comments : comments
    tasks ||--o{ task_events : audits
    tasks ||--o{ task_attachments : files
    users ||--o{ notifications : receives
```

### 5.1 Bảng

  -----------------------------------------------------------------------
  Bảng                                Mục đích
  ----------------------------------- -----------------------------------
  `organizations`                     Đơn vị sử dụng hệ thống

  `departments`                       Cây phòng/ban/tổ

  `users`                             Tài khoản

  `roles`                             Danh mục vai trò

  `user_roles`                        Vai trò của user trong đơn vị

  `department_members`                Quan hệ thành viên, quản lý, thời
                                      gian hiệu lực

  `tasks`                             Công việc và cây cha-con

  `task_assignments`                  Giao cho user/phòng, vai trò và
                                      người chịu trách nhiệm

  `task_updates`                      Các lần báo cáo tiến độ

  `task_reviews`                      Các lần nghiệm thu/yêu cầu bổ sung

  `task_comments`                     Trao đổi

  `task_attachments`                  Metadata tệp

  `task_events`                       Audit log nghiệp vụ

  `notifications`                     Thông báo người nhận

  `refresh_tokens`                    Phiên refresh token đã hash

  `idempotency_keys`                  Chống request tạo trùng
  -----------------------------------------------------------------------

### 5.2 Kiểu dữ liệu và ràng buộc

-   **PK/FK:** UUID v4.
-   **Optimistic Locking:** Bảng `tasks` có cột `version BIGINT NOT NULL DEFAULT 1`. Mọi thao tác cập nhật trạng thái/metadata đều đối soát và tăng `version = version + 1`.
-   **Thời gian:** `TIMESTAMPTZ`, lưu UTC, hiển thị theo `Asia/Ho_Chi_Minh`.
-   **Tiến độ:** `SMALLINT CHECK (progress BETWEEN 0 AND 100)`.
-   **Trọng số:** `INTEGER CHECK (weight > 0) DEFAULT 1`.
-   **Metadata linh hoạt:** `JSONB` chỉ dùng cho payload/audit (`task_events`, `idempotency_keys`), không thay thế quan hệ cần truy vấn.
-   **`task_assignments`:**
    -   Chính xác một trong `assignee_user_id` và `assignee_department_id` phải khác NULL:
        `CHECK ((assignee_user_id IS NOT NULL AND assignee_department_id IS NULL) OR (assignee_user_id IS NULL AND assignee_department_id IS NOT NULL))`.
    -   Khi giao cho phòng ban (`assignee_department_id IS NOT NULL`), có thể chỉ định `responsible_user_id` để xác định người đại diện phòng chịu trách nhiệm chính.
-   **Phân định Xóa và Hủy:**
    -   Không soft-delete nhiệm vụ đã có lịch sử thực hiện; hủy qua nghiệp vụ với `status = 'cancelled'`.
    -   `DeletedAt` chỉ dùng xóa bản nháp (`draft`) chưa giao việc.
-   **Khóa ngoại:** Đảm bảo toàn vẹn dữ liệu cùng tổ chức (`organization_id`); kiểm tra service và constraint phù hợp.
-   **Unique Index:** Tạo unique index theo tổ chức cho mã phòng, username, mã công việc (`organization_id`, `code`).
-   **Audit Log:** `task_events` là append-only, tuyệt đối không cho phép sửa hoặc xóa.

### 5.3 Index gợi ý

``` sql
CREATE INDEX idx_tasks_org_status_due
    ON tasks (organization_id, status, due_at);

CREATE INDEX idx_tasks_parent_status
    ON tasks (parent_id, status);

CREATE INDEX idx_tasks_root
    ON tasks (root_task_id);

CREATE INDEX idx_assignments_task
    ON task_assignments (task_id);

CREATE INDEX idx_assignments_user
    ON task_assignments (assignee_user_id);

CREATE INDEX idx_notifications_recipient_read
    ON notifications (recipient_id, read_at);
```

Các index phải được tạo trong migration sau khi chốt tên bảng/cột và
kiểm tra query thực tế.

## 6. Struct GORM

Các đoạn dưới đây là model lõi minh họa. Cần bổ sung foreign key,
unique/check constraints và DTO trước khi triển khai production.

### 6.1 Base model và tổ chức

``` go
package model

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

type BaseModel struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
    if m.ID == uuid.Nil {
        m.ID = uuid.New()
    }
    return nil
}

type Organization struct {
    BaseModel
    Code     string `gorm:"size:50;not null;uniqueIndex" json:"code"`
    Name     string `gorm:"size:255;not null" json:"name"`
    IsActive bool   `gorm:"not null;default:true" json:"is_active"`
}

type Department struct {
    BaseModel
    OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
    ParentID       *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
    Code           string     `gorm:"size:50;not null" json:"code"`
    Name           string     `gorm:"size:255;not null" json:"name"`
    DepartmentType string     `gorm:"size:30;not null" json:"department_type"`
    IsActive       bool       `gorm:"not null;default:true" json:"is_active"`
}

type User struct {
    BaseModel
    OrganizationID uuid.UUID `gorm:"type:uuid;not null;index" json:"organization_id"`
    Username       string    `gorm:"size:100;not null" json:"username"`
    Email          string    `gorm:"size:255" json:"email,omitempty"`
    PasswordHash   string    `gorm:"size:255;not null" json:"-"`
    FullName       string    `gorm:"size:255;not null" json:"full_name"`
    IsActive       bool      `gorm:"not null;default:true" json:"is_active"`
}

type Role struct {
    BaseModel
    Code        string `gorm:"size:50;not null;uniqueIndex" json:"code"`
    Name        string `gorm:"size:100;not null" json:"name"`
    Description string `gorm:"type:text" json:"description,omitempty"`
}

type UserRole struct {
    BaseModel
    OrganizationID uuid.UUID `gorm:"type:uuid;not null;index" json:"organization_id"`
    UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
    RoleID         uuid.UUID `gorm:"type:uuid;not null;index" json:"role_id"`
}

type DepartmentMember struct {
    BaseModel
    OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
    DepartmentID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"department_id"`
    UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
    ManagerUserID  *uuid.UUID `gorm:"type:uuid;index" json:"manager_user_id,omitempty"`
    PositionTitle  string     `gorm:"size:100" json:"position_title,omitempty"`
    IsPrimary      bool       `gorm:"not null;default:false" json:"is_primary"`
    EffectiveFrom  time.Time  `gorm:"not null" json:"effective_from"`
    EffectiveTo    *time.Time `json:"effective_to,omitempty"`
}
```

### 6.2 Task và assignment

``` go
package model

import (
    "time"

    "github.com/google/uuid"
)

type Task struct {
    BaseModel
    OrganizationID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
    ParentID         *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
    RootTaskID       *uuid.UUID `gorm:"type:uuid;index" json:"root_task_id,omitempty"`
    Code             string     `gorm:"size:50;not null;index" json:"code"`
    Title            string     `gorm:"size:255;not null" json:"title"`
    Description      string     `gorm:"type:text" json:"description,omitempty"`
    Status           string     `gorm:"size:30;not null;index;default:assigned" json:"status"`
    Priority         int16      `gorm:"not null;default:2" json:"priority"`
    Progress         int16      `gorm:"not null;default:0" json:"progress"`
    StartAt          *time.Time `json:"start_at,omitempty"`
    DueAt            *time.Time `gorm:"index" json:"due_at,omitempty"`
    CreatedBy        uuid.UUID  `gorm:"type:uuid;not null;index" json:"created_by"`
    ApprovalRequired bool       `gorm:"not null;default:true" json:"approval_required"`
    Weight           int32      `gorm:"not null;default:1" json:"weight"`
    Version          int64      `gorm:"not null;default:1" json:"version"`
    Children         []Task     `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

type TaskAssignment struct {
    BaseModel
    OrganizationID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
    TaskID               uuid.UUID  `gorm:"type:uuid;not null;index" json:"task_id"`
    AssigneeUserID       *uuid.UUID `gorm:"type:uuid;index" json:"assignee_user_id,omitempty"`
    AssigneeDepartmentID *uuid.UUID `gorm:"type:uuid;index" json:"assignee_department_id,omitempty"`
    ResponsibleUserID    *uuid.UUID `gorm:"type:uuid;index" json:"responsible_user_id,omitempty"`
    IsPrimary            bool       `gorm:"not null;default:false" json:"is_primary"`
    AssignmentRole       string     `gorm:"size:30;not null;default:responsible" json:"assignment_role"`
    AssignedBy           uuid.UUID  `gorm:"type:uuid;not null" json:"assigned_by"`
    Note                 string     `gorm:"type:text" json:"note,omitempty"`
}
```

`RootTaskID` của nhiệm vụ gốc có thể trỏ về chính ID của nó. Tạo ID
trước rồi thiết lập root trong service. Không preload cây vô hạn.
`TaskAssignment` có CHECK constraint đảm bảo chính xác 1 trong 2 loại đối
tượng nhận việc; nếu giao cho phòng ban, `ResponsibleUserID` lưu người đại
diện phụ trách chính.

### 6.3 Báo cáo, nghiệm thu, bình luận và audit

``` go
package model

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/datatypes"
)

type TaskUpdate struct {
    BaseModel
    TaskID      uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
    SubmittedBy uuid.UUID `gorm:"type:uuid;not null" json:"submitted_by"`
    Progress    int16     `gorm:"not null" json:"progress"`
    Content     string    `gorm:"type:text;not null" json:"content"`
    SubmittedAt time.Time `gorm:"not null" json:"submitted_at"`
}

type TaskReview struct {
    BaseModel
    TaskID     uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
    UpdateID   uuid.UUID `gorm:"type:uuid;not null" json:"update_id"`
    ReviewerID uuid.UUID `gorm:"type:uuid;not null" json:"reviewer_id"`
    Decision   string    `gorm:"size:30;not null" json:"decision"`
    Comment    string    `gorm:"type:text" json:"comment,omitempty"`
    ReviewedAt time.Time `gorm:"not null" json:"reviewed_at"`
}

type TaskComment struct {
    BaseModel
    TaskID   uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
    AuthorID uuid.UUID `gorm:"type:uuid;not null" json:"author_id"`
    Content  string    `gorm:"type:text;not null" json:"content"`
}

type TaskEvent struct {
    ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
    OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
    TaskID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"task_id"`
    ActorID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"actor_id"`
    EventType      string         `gorm:"size:50;not null" json:"event_type"`
    Data           datatypes.JSON `gorm:"type:jsonb;not null" json:"data"`
    CreatedAt      time.Time      `gorm:"not null;index" json:"created_at"`
}
```

`TaskReview.Decision` giới hạn `approved` và `revision_required`. Audit
event không được user thường sửa/xóa. Với yêu cầu mở lại, ghi event
`task.reopened` có `reason`, trạng thái trước/sau, actor và thời điểm.

### 6.4 Tệp và thông báo

``` go
package model

import (
    "time"

    "github.com/google/uuid"
)

type TaskAttachment struct {
    BaseModel
    TaskID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"task_id"`
    UpdateID    *uuid.UUID `gorm:"type:uuid;index" json:"update_id,omitempty"`
    UploadedBy  uuid.UUID  `gorm:"type:uuid;not null" json:"uploaded_by"`
    FileName    string     `gorm:"size:255;not null" json:"file_name"`
    ObjectKey   string     `gorm:"size:1024;not null" json:"-"`
    ContentType string     `gorm:"size:150" json:"content_type,omitempty"`
    SizeBytes   int64      `gorm:"not null" json:"size_bytes"`
    Checksum    string     `gorm:"size:128" json:"checksum,omitempty"`
    Status      string     `gorm:"size:20;not null;default:pending" json:"status"` // pending -> active
}

type Notification struct {
    BaseModel
    OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
    RecipientID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"recipient_id"`
    TaskID         *uuid.UUID `gorm:"type:uuid;index" json:"task_id,omitempty"`
    Type           string     `gorm:"size:50;not null" json:"type"`
    Title          string     `gorm:"size:255;not null" json:"title"`
    Body           string     `gorm:"type:text" json:"body,omitempty"`
    ReadAt         *time.Time `json:"read_at,omitempty"`
}
```

### 6.5 Phiên xác thực và Idempotency

``` go
package model

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/datatypes"
)

type RefreshToken struct {
    BaseModel
    UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
    TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
    ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
    RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type IdempotencyKey struct {
    Key            string         `gorm:"size:100;primaryKey" json:"key"`
    OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
    UserID         uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
    RequestMethod  string         `gorm:"size:10;not null" json:"request_method"`
    RequestPath    string         `gorm:"size:255;not null" json:"request_path"`
    ResponseCode   int            `gorm:"not null" json:"response_code"`
    ResponseBody   datatypes.JSON `gorm:"type:jsonb;not null" json:"response_body"`
    CreatedAt      time.Time      `gorm:"not null" json:"created_at"`
    ExpiresAt      time.Time      `gorm:"not null;index" json:"expires_at"`
}
```

Tệp lưu ngoài PostgreSQL (MinIO/S3), DB chỉ giữ metadata và object key.
`IdempotencyKey` hết hạn sau 24 giờ và được cronjob dọn dẹp định kỳ.

## 7. REST API contract

### 7.1 Quy ước

Base path: `/api/v1`

Headers:

``` http
Authorization: Bearer <access_token>
Content-Type: application/json
```

-   Dùng danh từ số nhiều cho resource: `/tasks`, `/users`,
    `/departments`.
-   GET đọc dữ liệu; POST tạo resource hoặc thực hiện command nghiệp vụ;
    PUT thay thế resource; PATCH cập nhật một phần; DELETE xóa hoặc
    ngừng resource theo chính sách.
-   Không dùng `GET` để thay đổi dữ liệu.
-   Request dùng DTO, không bind trực tiếp vào GORM model.
-   Dùng UUID và ISO 8601; lưu UTC, hiển thị múi giờ Việt Nam.
-   Pagination: `page`, `page_size`, giới hạn tối đa 100.
-   Hỗ trợ `Idempotency-Key` cho thao tác tạo quan trọng.
-   Dùng ETag/`If-Match` hoặc trường `version` để tránh ghi đè đồng
    thời.
-   Các hành động nghiệp vụ đặc biệt như submit, review, reopen có thể
    dùng resource con để giữ ngữ nghĩa REST rõ ràng.

### 7.2 Auth và người dùng

  Method   Endpoint                  Ý nghĩa
  -------- ------------------------- ------------------------------------------
  POST     `/auth/login`             Đăng nhập
  POST     `/auth/refresh`           Làm mới token
  POST     `/auth/logout`            Thu hồi phiên
  GET      `/auth/me`                Người dùng hiện tại và quyền
  GET      `/users`                  Danh sách người dùng trong phạm vi quyền
  POST     `/users`                  Tạo người dùng
  GET      `/users/{userId}`         Chi tiết
  PATCH    `/users/{userId}`         Cập nhật
  GET      `/users/{userId}/roles`   Xem vai trò
  PUT      `/users/{userId}/roles`   Thay thế tập vai trò theo quyền

### 7.3 Tổ chức

  --------------------------------------------------------------------------------------------------
  Method                  Endpoint                                           Ý nghĩa
  ----------------------- -------------------------------------------------- -----------------------
  GET                     `/organizations/current`                           Đơn vị hiện tại

  GET                     `/departments/tree`                                Cây tổ chức

  POST                    `/departments`                                     Tạo phòng/tổ

  GET                     `/departments/{departmentId}`                      Chi tiết

  PATCH                   `/departments/{departmentId}`                      Cập nhật

  GET                     `/departments/{departmentId}/members`              Thành viên

  POST                    `/departments/{departmentId}/members`              Thêm thành viên

  PATCH                   `/departments/{departmentId}/members/{memberId}`   Cập nhật quan hệ

  DELETE                  `/departments/{departmentId}/members/{memberId}`   Kết thúc quan hệ thành
                                                                             viên
  --------------------------------------------------------------------------------------------------

### 7.4 Công việc

  ----------------------------------------------------------------------------------------------
  Method                  Endpoint                                       Ý nghĩa
  ----------------------- ---------------------------------------------- -----------------------
  POST                    `/tasks`                                       Tạo nhiệm vụ gốc

  GET                     `/tasks`                                       Danh sách có bộ lọc

  GET                     `/tasks/{taskId}`                              Chi tiết

  PATCH                   `/tasks/{taskId}`                              Sửa trường được phép

  DELETE                  `/tasks/{taskId}`                              Xóa mềm/hủy theo chính
                                                                         sách; không xóa lịch sử

  GET                     `/tasks/{taskId}/children`                     Danh sách con

  POST                    `/tasks/{taskId}/children`                     Tạo công việc con

  GET                     `/tasks/{taskId}/tree`                         Lấy cây có giới hạn độ
                                                                         sâu

  GET                     `/tasks/{taskId}/assignments`                  Người/phòng được giao

  POST                    `/tasks/{taskId}/assignments`                  Tạo giao việc

  PATCH                   `/tasks/{taskId}/assignments/{assignmentId}`   Thay đổi assignment
                                                                         theo quyền

  GET                     `/tasks/{taskId}/updates`                      Lịch sử báo cáo

  POST                    `/tasks/{taskId}/updates`                      Tạo báo cáo tiến độ

  GET                     `/tasks/{taskId}/reviews`                      Lịch sử nghiệm thu

  POST                    `/tasks/{taskId}/reviews`                      Tạo quyết định nghiệm
                                                                         thu

  GET                     `/tasks/{taskId}/comments`                     Bình luận

  POST                    `/tasks/{taskId}/comments`                     Tạo bình luận

  GET                     `/tasks/{taskId}/attachments`                  Danh sách tệp

  POST                    `/tasks/{taskId}/attachments/presign`          Khởi tạo Presigned URL upload tệp (MinIO/S3)

  POST                    `/tasks/{taskId}/attachments/confirm`          Xác nhận upload thành công (active attachment)

  GET                     `/tasks/{taskId}/attachments/{attachmentId}/download`  Lấy Presigned URL tải tệp bảo mật

  DELETE                  `/tasks/{taskId}/attachments/{attachmentId}`   Gỡ tệp theo quyền

  GET                     `/tasks/{taskId}/events`                       Audit timeline

  POST                    `/tasks/{taskId}/reopen`                       Mở lại công việc đã
                                                                         nghiệm thu, bắt buộc lý
                                                                         do

  POST                    `/tasks/{taskId}/cancellations`                Tạo yêu cầu/thực hiện
                                                                         hủy có lý do
  ----------------------------------------------------------------------------------------------

Các endpoint con là resource lồng nhau để thể hiện quan hệ; nếu sau này
cần truy cập trực tiếp, có thể bổ sung `/task-updates/{updateId}` hoặc
`/task-reviews/{reviewId}`.

### 7.5 Dashboard, báo cáo, thông báo

  Method   Endpoint                            Ý nghĩa
  -------- ----------------------------------- ------------------------------
  GET      `/dashboard/summary`                Tổng quan theo quyền
  GET      `/dashboard/my-tasks`               Công việc của người hiện tại
  GET      `/reports/tasks`                    Báo cáo công việc
  GET      `/reports/departments`              Báo cáo phòng ban
  POST     `/report-exports`                   Tạo yêu cầu xuất
  GET      `/report-exports/{exportId}`        Trạng thái/kết quả xuất
  GET      `/notifications`                    Danh sách thông báo
  GET      `/notifications/unread-count`       Số thông báo chưa đọc
  PATCH    `/notifications/{notificationId}`   Đánh dấu đã đọc
  POST     `/notifications/read-all`           Đánh dấu tất cả đã đọc
  GET      `/notifications/stream`             SSE (Server-Sent Events) đẩy thông báo thời gian thực

### 7.6 Tạo nhiệm vụ

`POST /api/v1/tasks`

``` json
{
  "title": "Triển khai kế hoạch quý IV",
  "description": "Tổ chức thực hiện các nhiệm vụ trọng tâm",
  "priority": 1,
  "start_at": "2026-11-01T08:00:00+07:00",
  "due_at": "2026-11-30T17:00:00+07:00",
  "approval_required": true,
  "initial_assignment": {
    "type": "department",
    "id": "UUID_PHONG_BAN",
    "responsible_user_id": "UUID_TRUONG_PHONG"
  }
}
```

Response `201 Created`:

``` json
{
  "data": {
    "id": "UUID_TASK",
    "code": "TASK-2026-00001",
    "title": "Triển khai kế hoạch quý IV",
    "parent_id": null,
    "root_task_id": "UUID_TASK",
    "status": "assigned",
    "priority": 1,
    "progress": 0,
    "version": 1,
    "due_at": "2026-11-30T10:00:00Z",
    "created_at": "2026-10-29T09:30:00Z"
  }
}
```

Tạo task và assignment trong cùng transaction. `created_by`,
`organization_id`, `root_task_id` được server xác định. Mã công việc cần
sinh an toàn khi đồng thời, ví dụ sequence theo đơn vị/năm.

### 7.7 Tạo công việc con

`POST /api/v1/tasks/{taskId}/children`

``` json
{
  "title": "Tổng hợp số liệu quý IV",
  "description": "Thu thập và kiểm tra số liệu",
  "priority": 2,
  "start_at": "2026-11-01T08:00:00+07:00",
  "due_at": "2026-11-05T17:00:00+07:00",
  "weight": 2,
  "approval_required": true,
  "initial_assignment": {
    "type": "user",
    "id": "UUID_NHAN_VIEN"
  }
}
```

Backend lấy `parent_id` từ URL, tính `root_task_id`, lấy `created_by` từ
người đăng nhập và kiểm tra quyền, cùng đơn vị, chu kỳ cây, trạng thái
cha và thời hạn. Với quyết định đã chốt, thời gian con phải nằm trong
khoảng thời gian của cha; nếu cha có hạn thì con không được vượt hạn.

### 7.8 Danh sách và lọc

Ví dụ:

``` http
GET /api/v1/tasks?page=1&page_size=20&status=in_progress&priority=1&department_id=UUID&sort_by=due_at&sort_order=asc
```

Query parameters:

  Param                            Mô tả
  -------------------------------- ---------------------------------
  `page`, `page_size`              Phân trang
  `q`                              Tìm mã/tiêu đề
  `status`, `priority`             Trạng thái, mức ưu tiên
  `assignee_id`, `department_id`   Người/phòng được giao
  `parent_id`, `root_task_id`      Lọc cây
  `due_before`, `due_after`        Khoảng hạn
  `overdue`                        Chỉ quá hạn
  `sort_by`, `sort_order`          Sắp xếp theo danh sách cho phép

Response:

``` json
{
  "data": [],
  "meta": {
    "page": 1,
    "page_size": 20,
    "total": 0,
    "total_pages": 0
  }
}
```

Tổng số và danh sách phải cùng áp dụng phạm vi quyền. Chỉ cho phép sort
theo danh sách cột whitelist, không nối trực tiếp chuỗi query client vào
SQL.

### 7.9 Submit và review

Tạo báo cáo:

`POST /api/v1/tasks/{taskId}/updates`

``` json
{
  "progress": 100,
  "content": "Đã hoàn thành thu thập và kiểm tra số liệu",
  "attachment_ids": ["UUID_ATTACHMENT_1"]
}
```

Tạo nghiệm thu:

`POST /api/v1/tasks/{taskId}/reviews`

``` json
{
  "update_id": "UUID_UPDATE",
  "decision": "approved",
  "comment": "Đã kiểm tra và thống nhất kết quả"
}
```

Hoặc:

``` json
{
  "update_id": "UUID_UPDATE",
  "decision": "revision_required",
  "comment": "Bổ sung bảng đối chiếu số liệu"
}
```

`reviewer_id`, `reviewed_at` và trạng thái mới do server xác định. Kiểm
tra người review có quyền, update thuộc task, và điều kiện hoàn thành
của các công việc con.

### 7.10 Mở lại công việc

`POST /api/v1/tasks/{taskId}/reopen`

``` json
{
  "reason": "Phát hiện cần bổ sung số liệu sau nghiệm thu"
}
```

-   `reason` bắt buộc, không rỗng.
-   Chỉ reviewer gần nhất hoặc cấp quản lý cao hơn được phép.
-   Khóa task trong transaction, kiểm tra trạng thái hiện tại và quyền.
-   Không xóa review cũ.
-   Ghi `task_events` với `event_type = task.reopened`, lý do, actor,
    trạng thái trước/sau và thời điểm.
-   Nếu cần mở lại cha, phải ghi sự kiện riêng cho mỗi công việc được mở
    lại.
-   Nếu quyền hoặc trạng thái không hợp lệ, trả lỗi phù hợp, không sửa
    dữ liệu.

### 7.11 Mã lỗi HTTP

  -----------------------------------------------------------------------
  HTTP                                Dùng cho
  ----------------------------------- -----------------------------------
  `400 Bad Request`                   JSON/query sai

  `401 Unauthorized`                  Chưa xác thực/token sai

  `403 Forbidden`                     Không đủ quyền

  `404 Not Found`                     Không tìm thấy hoặc không được phép
                                      nhìn thấy

  `409 Conflict`                      Xung đột trạng thái, cập nhật đồng
                                      thời, hạn cha-con không hợp lệ

  `422 Unprocessable Content`         Dữ liệu hợp lệ về cấu trúc nhưng
                                      sai nghiệp vụ

  `429 Too Many Requests`             Rate limit

  `500 Internal Server Error`         Lỗi không dự kiến
  -----------------------------------------------------------------------

Response lỗi:

``` json
{
  "error": {
    "code": "TASK_CHILD_DUE_DATE_OUT_OF_RANGE",
    "message": "Hạn hoàn thành công việc con không được vượt hạn công việc cha.",
    "details": {
      "parent_due_at": "2026-11-30T10:00:00Z",
      "requested_due_at": "2026-12-01T10:00:00Z"
    },
    "request_id": "req-identifier"
  }
}
```

## 8. DTO Go cho API

``` go
package tasks

import (
    "time"

    "github.com/google/uuid"
)

type AssignmentInput struct {
    Type              string     `json:"type" validate:"required,oneof=user department"`
    ID                uuid.UUID  `json:"id" validate:"required"`
    ResponsibleUserID *uuid.UUID `json:"responsible_user_id,omitempty"`
}

type CreateTaskRequest struct {
    Title             string          `json:"title" validate:"required,max=255"`
    Description       string          `json:"description"`
    Priority          int16           `json:"priority" validate:"gte=1,lte=3"`
    StartAt           *time.Time      `json:"start_at"`
    DueAt             *time.Time      `json:"due_at"`
    ApprovalRequired  *bool           `json:"approval_required"`
    InitialAssignment AssignmentInput `json:"initial_assignment" validate:"required"`
}

type CreateChildTaskRequest struct {
    Title             string          `json:"title" validate:"required,max=255"`
    Description       string          `json:"description"`
    Priority          int16           `json:"priority" validate:"gte=1,lte=3"`
    StartAt           *time.Time      `json:"start_at"`
    DueAt             *time.Time      `json:"due_at"`
    Weight            int32           `json:"weight" validate:"gte=1"`
    ApprovalRequired  *bool           `json:"approval_required"`
    InitialAssignment AssignmentInput `json:"initial_assignment" validate:"required"`
}

type UpdateTaskRequest struct {
    Title       *string    `json:"title,omitempty" validate:"omitempty,max=255"`
    Description *string    `json:"description,omitempty"`
    Priority    *int16     `json:"priority,omitempty" validate:"omitempty,gte=1,lte=3"`
    DueAt       *time.Time `json:"due_at,omitempty"`
    Version     int64      `json:"version" validate:"required,gte=1"` // Optimistic Locking
}

type CreateTaskUpdateRequest struct {
    Progress      int16       `json:"progress" validate:"gte=0,lte=100"`
    Content       string      `json:"content" validate:"required"`
    AttachmentIDs []uuid.UUID `json:"attachment_ids"`
}

type CreateTaskReviewRequest struct {
    UpdateID uuid.UUID `json:"update_id" validate:"required"`
    Decision string    `json:"decision" validate:"required,oneof=approved revision_required"`
    Comment  string    `json:"comment"`
}

type ReopenTaskRequest struct {
    Reason string `json:"reason" validate:"required,min=5,max=2000"`
}

type PresignUploadRequest struct {
    FileName    string `json:"file_name" validate:"required,max=255"`
    ContentType string `json:"content_type" validate:"required"`
    SizeBytes   int64  `json:"size_bytes" validate:"required,gt=0"`
}

type PresignUploadResponse struct {
    AttachmentID uuid.UUID `json:"attachment_id"`
    UploadURL    string    `json:"upload_url"`
    ObjectKey    string    `json:"object_key"`
    ExpiresInSec int       `json:"expires_in_sec"`
}

type ConfirmUploadRequest struct {
    AttachmentID uuid.UUID `json:"attachment_id" validate:"required"`
    Checksum     string    `json:"checksum,omitempty"`
}
```

Các tag validator không tự chạy; handler phải gọi validator. Validation
nghiệp vụ (quyền, hạn cha-con, người nghiệm thu) nằm trong service.

## 9. Cấu trúc source code

``` text
backend/
├── cmd/api/main.go
├── internal/
│   ├── config/
│   ├── database/
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── organization.go
│   │   └── permission.go
│   ├── modules/
│   │   ├── auth/
│   │   ├── organization/
│   │   ├── users/
│   │   ├── tasks/
│   │   │   ├── model.go
│   │   │   ├── dto.go
│   │   │   ├── repository.go
│   │   │   ├── service.go
│   │   │   ├── handler.go
│   │   │   └── routes.go
│   │   ├── reports/
│   │   └── notifications/
│   ├── platform/
│   │   ├── errors/
│   │   ├── response/
│   │   └── audit/
│   └── router/
├── migrations/
├── tests/
├── Dockerfile
├── compose.yaml
└── go.mod
```

Handler Fiber chỉ xử lý HTTP và DTO. Service chứa nghiệp vụ/transaction;
repository chứa truy vấn. Không để handler truy cập GORM trực tiếp.

## 10. Transaction, concurrency và audit

Các thao tác bắt buộc chạy trong Database Transaction:

-   Tạo task và assignment.
-   Tạo child task và ghi event audit.
-   Thay đổi người phụ trách / phòng ban.
-   Submit báo cáo tiến độ và kích hoạt tính toán tổng hợp tiến độ cha.
-   Review và chuyển trạng thái nghiệm thu.
-   Reopen task.
-   Hủy task (chuyển `cancelled` và xử lý ràng buộc nhánh con).

**Kiểm soát Concurrency và Deadlock:**
1.  **Optimistic Locking:** Với các thao tác sửa thông tin cơ bản hoặc trạng thái của công việc:
    `UPDATE tasks SET ..., version = version + 1 WHERE id = ? AND version = ?;`
    Nếu số hàng ảnh hưởng bằng 0 (`RowsAffected == 0`), Backend từ chối với mã lỗi `409 Conflict`, buộc client reload dữ liệu mới nhất.
2.  **Pessimistic Locking & Chống Deadlock Rollup Tiến độ:**
    -   Khi nghiệm thu, mở lại hoặc sửa hạn, sử dụng `SELECT ... FOR UPDATE` có thời gian timeout ngắn (`SET LOCAL lock_timeout = '3s'`).
    -   Khi nhiều công việc con cùng cập nhật tiến độ đồng thời, việc tính toán tổng hợp tiến độ lên các cấp cha (tổ tiên) có thể gây xung đột lock. Để chống Deadlock: các transaction cập nhật chuỗi tổ tiên phải luôn khóa tài nguyên theo **thứ tự khóa nhất quán (khóa theo thứ tự ID tăng dần `ORDER BY id ASC`)**. Với cây công việc lớn, tiến độ cha có thể được tổng hợp thông qua kênh event nội bộ (transactional outbox) nhằm giải phóng lock nhanh chóng.

**Audit Event:**
Mọi thay đổi trạng thái quan trọng bắt buộc ghi `task_events` append-only với actor, loại sự kiện, thời gian, trạng thái trước/sau và lý do. Tuyệt đối không cho phép cập nhật hay xóa bản ghi `task_events`.

## 11. Bảo mật và kiểm thử bắt buộc

1.  Nhân viên phòng A không đọc được nhiệm vụ riêng phòng B.
2.  Không thể giả mạo `created_by`, `organization_id`, reviewer hoặc
    root ID.
3.  Công việc con không vượt hạn cha; sửa hạn cha bị từ chối nếu làm con
    vượt hạn.
4.  Không tạo được chu trình trong cây.
5.  Không nghiệm thu khi công việc con bắt buộc chưa hoàn thành.
6.  Chỉ reviewer gần nhất hoặc cấp quản lý cao hơn được reopen.
7.  Reopen không có lý do phải bị từ chối.
8.  Mọi lần reopen đều có audit log và không xóa review cũ.
9.  Request tạo bị gửi lại không sinh bản ghi trùng khi dùng cùng
    `Idempotency-Key`.
10. Hai request cập nhật đồng thời không ghi đè âm thầm.
11. File download kiểm tra quyền.
12. Báo cáo chỉ tổng hợp dữ liệu thuộc phạm vi quyền.

## 12. Migration

Thứ tự tạo bảng đề xuất:

1.  `organizations`
2.  `departments`
3.  `users`
4.  `roles`, `user_roles`
5.  `department_members`
6.  `tasks`
7.  `task_assignments`
8.  `task_updates`, `task_reviews`
9.  `task_comments`, `task_attachments`
10. `task_events`, `notifications`
11. `refresh_tokens`, `idempotency_keys`

Dùng migration có version trong production. `AutoMigrate` chỉ phù hợp hỗ
trợ phát triển ban đầu; cần migration SQL rõ ràng cho foreign key, check
constraint, unique index, dữ liệu chuyển đổi và các thay đổi schema.

## 13. Kế hoạch triển khai

  Giai đoạn   Nội dung
  ----------- ----------------------------------------------------------
  1           Chốt nghiệp vụ, ERD, API contract, quyền
  2           Fiber skeleton, config, PostgreSQL, migration, auth
  3           Tổ chức, người dùng, membership, role
  4           Task CRUD, cây công việc, assignment, ràng buộc thời hạn
  5           Tiến độ, updates, review, reopen và audit
  6           Frontend Nuxt 4, dashboard, cây công việc, biểu mẫu
  7           Notifications, báo cáo, xuất dữ liệu
  8           Kiểm thử bảo mật, UAT, backup/restore, triển khai

Ước lượng MVP cho một Backend developer và một Frontend developer:
khoảng 8--12 tuần tùy yêu cầu thực tế và độ phức tạp phân quyền.

## 14. Tiêu chí nghiệm thu MVP

-   Chỉ huy giao việc cho phòng; trưởng phòng giao cho nhân viên; cấp
    dưới tạo công việc con theo quyền.
-   Xem được cây công việc và tiến độ tổng hợp.
-   Hạn công việc con nằm trong hạn cha; mọi ngoại lệ phải có quyền, lý
    do và audit.
-   Nghiệm thu đúng thẩm quyền; yêu cầu bổ sung được xử lý đúng trạng
    thái.
-   Chỉ reviewer gần nhất hoặc cấp quản lý cao hơn được mở lại; lý do
    bắt buộc và audit log đầy đủ.
-   Không truy cập trái phép bằng cách sửa UUID trên API.
-   Dashboard và báo cáo khớp với dữ liệu trong phạm vi quyền.
-   Có migration, log, backup và đã thử khôi phục.
