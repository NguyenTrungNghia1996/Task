package departments

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	_ "taskmgmt/internal/model"
	"taskmgmt/internal/platform/response"
)

type Handler struct {
	svc      *Service
	validate *validator.Validate
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc:      svc,
		validate: validator.New(),
	}
}

// GetCurrentOrganization godoc
// @Summary Thông tin đơn vị hiện tại
// @Description Lấy thông tin đơn vị tổ chức của người dùng hiện tại
// @Tags Organizations
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=model.Organization}
// @Router /organizations/current [get]
func (h *Handler) GetCurrentOrganization(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	org, err := h.svc.GetCurrentOrganization(orgID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_ORG_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, org)
}

// GetDepartmentTree godoc
// @Summary Cây cơ cấu tổ chức & phòng ban
// @Description Lấy danh sách phòng ban dạng cây phân cấp (tương thích Ant Design a-tree & a-tree-select)
// @Tags Departments
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]DepartmentTreeNode}
// @Router /departments/tree [get]
func (h *Handler) GetDepartmentTree(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	tree, err := h.svc.GetDepartmentTree(orgID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_TREE_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, tree)
}

// GetDepartmentOptions godoc
// @Summary Danh sách phòng ban dạng Options
// @Description Lấy danh sách phòng ban dạng options cho Select và TreeSelect của Ant Design
// @Tags Departments
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]DepartmentOption}
// @Router /departments/options [get]
func (h *Handler) GetDepartmentOptions(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	options, err := h.svc.GetDepartmentOptions(orgID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_OPTIONS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, options)
}

// GetDepartment godoc
// @Summary Chi tiết phòng ban
// @Description Xem thông tin chi tiết một phòng ban theo ID
// @Tags Departments
// @Security BearerAuth
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Success 200 {object} response.SuccessResponse{data=DepartmentDetailResponse}
// @Router /departments/{departmentId} [get]
func (h *Handler) GetDepartment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "ID phòng ban không đúng định dạng", nil)
	}

	detail, err := h.svc.GetDepartmentDetail(orgID, deptID)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_DETAIL_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, detail)
}

// CreateDepartment godoc
// @Summary Tạo phòng ban mới
// @Description Thêm phòng ban hoặc tổ/đội mới vào cơ cấu đơn vị
// @Tags Departments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateDepartmentRequest true "Thông tin phòng ban mới"
// @Success 201 {object} response.SuccessResponse{data=DepartmentDetailResponse}
// @Router /departments [post]
func (h *Handler) CreateDepartment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	var req CreateDepartmentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Thông tin phòng ban không hợp lệ", err.Error())
	}

	dept, err := h.svc.CreateDepartment(orgID, req)
	if err != nil {
		if errors.Is(err, ErrDepartmentCodeExists) {
			return response.Error(c, fiber.StatusConflict, "CODE_EXISTS", "Mã phòng ban đã tồn tại trong đơn vị", nil)
		}
		if errors.Is(err, ErrParentDepartmentNotFound) {
			return response.Error(c, fiber.StatusBadRequest, "PARENT_NOT_FOUND", "Phòng ban cha không tồn tại", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "CREATE_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, dept)
}

// UpdateDepartment godoc
// @Summary Cập nhật phòng ban
// @Description Sửa tên, mã, loại phòng ban hoặc chuyển phòng ban cha (chống lặp chu trình)
// @Tags Departments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Param request body UpdateDepartmentRequest true "Thông tin cập nhật"
// @Success 200 {object} response.SuccessResponse{data=DepartmentDetailResponse}
// @Router /departments/{departmentId} [patch]
func (h *Handler) UpdateDepartment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "ID phòng ban không đúng định dạng", nil)
	}

	var req UpdateDepartmentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Thông tin cập nhật không hợp lệ", err.Error())
	}

	dept, err := h.svc.UpdateDepartment(orgID, deptID, req)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		if errors.Is(err, ErrDepartmentCodeExists) {
			return response.Error(c, fiber.StatusConflict, "CODE_EXISTS", "Mã phòng ban đã tồn tại", nil)
		}
		if errors.Is(err, ErrDepartmentCycleDetected) {
			return response.Error(c, fiber.StatusConflict, "CYCLE_DETECTED", "Không thể chọn chính nó hoặc đơn vị con làm phòng ban cha", nil)
		}
		if errors.Is(err, ErrParentDepartmentNotFound) {
			return response.Error(c, fiber.StatusBadRequest, "PARENT_NOT_FOUND", "Phòng ban cha không tồn tại", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "UPDATE_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, dept)
}

// DeleteDepartment godoc
// @Summary Xóa phòng ban
// @Description Xóa phòng ban (chỉ xóa được khi không có phòng ban con và không còn thành viên đang hoạt động)
// @Tags Departments
// @Security BearerAuth
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Router /departments/{departmentId} [delete]
func (h *Handler) DeleteDepartment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "ID phòng ban không đúng định dạng", nil)
	}

	if err := h.svc.DeleteDepartment(orgID, deptID); err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		if errors.Is(err, ErrDepartmentHasChildren) {
			return response.Error(c, fiber.StatusConflict, "HAS_CHILDREN", "Không thể xóa phòng ban đang có đơn vị con", nil)
		}
		if errors.Is(err, ErrDepartmentHasMembers) {
			return response.Error(c, fiber.StatusConflict, "HAS_MEMBERS", "Không thể xóa phòng ban đang có nhân sự trực thuộc", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "DELETE_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"message": "Xóa phòng ban thành công",
	})
}

// GetDepartmentMembers godoc
// @Summary Danh sách thành viên phòng ban
// @Description Lấy danh sách nhân sự trực thuộc phòng ban kèm chức danh và vai trò
// @Tags Departments
// @Security BearerAuth
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]DepartmentMemberDTO}
// @Router /departments/{departmentId}/members [get]
func (h *Handler) GetDepartmentMembers(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "ID phòng ban không đúng định dạng", nil)
	}

	members, err := h.svc.GetDepartmentMembers(orgID, deptID)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_MEMBERS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, members)
}

// AddDepartmentMember godoc
// @Summary Thêm nhân sự vào phòng ban
// @Description Gán nhân sự vào phòng ban kèm chức danh và quản lý trực tiếp
// @Tags Departments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Param request body AddDepartmentMemberRequest true "Thông tin thành viên"
// @Success 201 {object} response.SuccessResponse{data=DepartmentMemberDTO}
// @Router /departments/{departmentId}/members [post]
func (h *Handler) AddDepartmentMember(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "ID phòng ban không đúng định dạng", nil)
	}

	var req AddDepartmentMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Thông tin thành viên không hợp lệ", err.Error())
	}

	member, err := h.svc.AddDepartmentMember(orgID, deptID, req)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "DEPT_NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		if errors.Is(err, ErrUserNotFound) {
			return response.Error(c, fiber.StatusNotFound, "USER_NOT_FOUND", "Không tìm thấy nhân sự trong đơn vị", nil)
		}
		if errors.Is(err, ErrMemberAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "MEMBER_EXISTS", "Nhân sự đã là thành viên của phòng ban này", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "ADD_MEMBER_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, member)
}

// UpdateDepartmentMember godoc
// @Summary Cập nhật thành viên phòng ban
// @Description Sửa chức danh, người quản lý hoặc thời hạn kiêm nhiệm
// @Tags Departments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Param memberId path string true "ID quan hệ thành viên (UUID)"
// @Param request body UpdateDepartmentMemberRequest true "Thông tin cập nhật"
// @Success 200 {object} response.SuccessResponse{data=DepartmentMemberDTO}
// @Router /departments/{departmentId}/members/{memberId} [patch]
func (h *Handler) UpdateDepartmentMember(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_DEPT_ID", "ID phòng ban không đúng định dạng", nil)
	}

	memberID, err := uuid.Parse(c.Params("memberId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_MEMBER_ID", "ID thành viên không đúng định dạng", nil)
	}

	var req UpdateDepartmentMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Thông tin cập nhật không hợp lệ", err.Error())
	}

	member, err := h.svc.UpdateDepartmentMember(orgID, deptID, memberID, req)
	if err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "DEPT_NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		if errors.Is(err, ErrMemberNotFound) {
			return response.Error(c, fiber.StatusNotFound, "MEMBER_NOT_FOUND", "Không tìm thấy thành viên", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "UPDATE_MEMBER_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, member)
}

// RemoveDepartmentMember godoc
// @Summary Rút nhân sự khỏi phòng ban
// @Description Kết thúc quan hệ thành viên phòng ban
// @Tags Departments
// @Security BearerAuth
// @Produce json
// @Param departmentId path string true "ID phòng ban (UUID)"
// @Param memberId path string true "ID thành viên (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Router /departments/{departmentId}/members/{memberId} [delete]
func (h *Handler) RemoveDepartmentMember(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	deptID, err := uuid.Parse(c.Params("departmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_DEPT_ID", "ID phòng ban không đúng định dạng", nil)
	}

	memberID, err := uuid.Parse(c.Params("memberId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_MEMBER_ID", "ID thành viên không đúng định dạng", nil)
	}

	if err := h.svc.RemoveDepartmentMember(orgID, deptID, memberID); err != nil {
		if errors.Is(err, ErrDepartmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "DEPT_NOT_FOUND", "Không tìm thấy phòng ban", nil)
		}
		if errors.Is(err, ErrMemberNotFound) {
			return response.Error(c, fiber.StatusNotFound, "MEMBER_NOT_FOUND", "Không tìm thấy thành viên", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "REMOVE_MEMBER_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"message": "Rút nhân sự khỏi phòng ban thành công",
	})
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	// Organizations
	orgs := router.Group("/organizations", authMiddleware)
	orgs.Get("/current", h.GetCurrentOrganization)

	// Departments
	depts := router.Group("/departments", authMiddleware)
	depts.Get("/tree", h.GetDepartmentTree)
	depts.Get("/options", h.GetDepartmentOptions)
	depts.Get("/:departmentId", h.GetDepartment)
	depts.Post("/", h.CreateDepartment)
	depts.Patch("/:departmentId", h.UpdateDepartment)
	depts.Delete("/:departmentId", h.DeleteDepartment)

	// Department Members
	depts.Get("/:departmentId/members", h.GetDepartmentMembers)
	depts.Post("/:departmentId/members", h.AddDepartmentMember)
	depts.Patch("/:departmentId/members/:memberId", h.UpdateDepartmentMember)
	depts.Delete("/:departmentId/members/:memberId", h.RemoveDepartmentMember)
}
