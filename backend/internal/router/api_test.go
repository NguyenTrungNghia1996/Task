package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"taskmgmt/internal/config"
	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/auth"
	"taskmgmt/internal/modules/departments"
	"taskmgmt/internal/modules/menus"
	"taskmgmt/internal/modules/rbac"
	"taskmgmt/internal/modules/tasks"
	"taskmgmt/internal/modules/users"
	"taskmgmt/internal/router"
	"taskmgmt/internal/testutil"
)

type apiTestContext struct {
	app        *fiber.App
	db         *gorm.DB
	org        *model.Organization
	admin      *model.User
	adminToken string
}

func setupAPITest(t *testing.T) *apiTestContext {
	t.Helper()

	db := testutil.SetupTestDB(t)
	cfg := &config.Config{
		JWTSecret:         "api-test-jwt-secret-very-secure-key",
		JWTAccessTTLMin:   15,
		JWTRefreshTTLDays: 7,
	}

	app := fiber.New()
	router.Setup(app, db, cfg)

	// Create test org and admin user
	org, admin := testutil.CreateTestOrgAndUser(t, db, "admin", "system_admin")

	// Login to obtain real access token
	authSvc := auth.NewService(db, cfg)
	loginRes, err := authSvc.Login("admin", "password123")
	require.NoError(t, err)

	return &apiTestContext{
		app:        app,
		db:         db,
		org:        org,
		admin:      admin,
		adminToken: loginRes.AccessToken,
	}
}

func doRequest(app *fiber.App, method, path string, body interface{}, token string) (*http.Response, map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req, 10000)
	if err != nil {
		return nil, nil, err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}

	var jsonMap map[string]interface{}
	if len(respBytes) > 0 {
		_ = json.Unmarshal(respBytes, &jsonMap)
	}

	return resp, jsonMap, nil
}

func doMultipartRequest(app *fiber.App, path, fieldName, fileName string, fileContent []byte, token string) (*http.Response, map[string]interface{}, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, nil, err
	}
	if _, err := io.Copy(part, bytes.NewReader(fileContent)); err != nil {
		return nil, nil, err
	}
	_ = writer.Close()

	req := httptest.NewRequest("POST", path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req, 10000)
	if err != nil {
		return nil, nil, err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}

	var data map[string]interface{}
	if len(respBytes) > 0 {
		_ = json.Unmarshal(respBytes, &data)
	}
	return resp, data, nil
}

func TestAPI_Health(t *testing.T) {
	ctx := setupAPITest(t)

	resp, data, err := doRequest(ctx.app, "GET", "/health", nil, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", data["data"].(map[string]interface{})["status"])
	assert.Equal(t, "v1.1", data["data"].(map[string]interface{})["version"])
}

func TestAPI_Auth_Login_And_Me(t *testing.T) {
	ctx := setupAPITest(t)

	// 1. POST /auth/login - valid
	loginBody := map[string]string{
		"username": "admin",
		"password": "password123",
	}
	resp, data, err := doRequest(ctx.app, "POST", "/api/v1/auth/login", loginBody, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	dataMap := data["data"].(map[string]interface{})
	token := dataMap["access_token"].(string)
	refreshToken := dataMap["refresh_token"].(string)
	assert.NotEmpty(t, token)
	assert.NotEmpty(t, refreshToken)

	// 2. POST /auth/login - wrong password -> 401
	badLogin := map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	}
	resp2, data2, err := doRequest(ctx.app, "POST", "/api/v1/auth/login", badLogin, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
	assert.NotNil(t, data2["error"])

	// 3. GET /auth/me - without token -> 401
	resp3, _, err := doRequest(ctx.app, "GET", "/api/v1/auth/me", nil, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp3.StatusCode)

	// 4. GET /auth/me - with valid token -> 200
	resp4, data4, err := doRequest(ctx.app, "GET", "/api/v1/auth/me", nil, token)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
	meData := data4["data"].(map[string]interface{})
	assert.Equal(t, "admin", meData["username"])

	// 5. POST /auth/refresh - valid refresh token -> 200
	refBody := map[string]string{"refresh_token": refreshToken}
	resp5, data5, err := doRequest(ctx.app, "POST", "/api/v1/auth/refresh", refBody, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp5.StatusCode)
	assert.NotEmpty(t, data5["data"].(map[string]interface{})["access_token"])
}

func TestAPI_Users_CRUD(t *testing.T) {
	ctx := setupAPITest(t)

	// 1. POST /users - Create User
	createReq := users.CreateUserRequest{
		Username:      "tran_nam",
		FullName:      "Trần Hoài Nam",
		Email:         "nam@test.com",
		Password:      "password123",
		PositionTitle: "Chuyên viên tác chiến",
	}
	resp, data, err := doRequest(ctx.app, "POST", "/api/v1/users", createReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	userData := data["data"].(map[string]interface{})
	userIDStr := userData["id"].(string)
	assert.NotEmpty(t, userIDStr)
	assert.Equal(t, "tran_nam", userData["username"])
	assert.Equal(t, "Trần Hoài Nam", userData["full_name"])

	// 2. GET /users - List with pagination
	resp2, data2, err := doRequest(ctx.app, "GET", "/api/v1/users?page=1&page_size=10", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	items := data2["data"].([]interface{})
	assert.GreaterOrEqual(t, len(items), 2) // admin + tran_nam
	meta := data2["meta"].(map[string]interface{})
	assert.Equal(t, float64(1), meta["page"])

	// 3. GET /users/options - Select options
	resp3, data3, err := doRequest(ctx.app, "GET", "/api/v1/users/options", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	options := data3["data"].([]interface{})
	assert.GreaterOrEqual(t, len(options), 2)

	// 4. GET /users/:userId - Detail
	resp4, data4, err := doRequest(ctx.app, "GET", "/api/v1/users/"+userIDStr, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
	assert.Equal(t, "tran_nam", data4["data"].(map[string]interface{})["username"])

	// 5. PATCH /users/:userId - Update
	newFullName := "Trần Hoài Nam (Đã cập nhật)"
	updateReq := users.UpdateUserRequest{
		FullName: &newFullName,
	}
	resp5, data5, err := doRequest(ctx.app, "PATCH", "/api/v1/users/"+userIDStr, updateReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp5.StatusCode)
	assert.Equal(t, newFullName, data5["data"].(map[string]interface{})["full_name"])

	// 6. POST /users/:userId/reset-password
	resetReq := users.ResetPasswordRequest{
		NewPassword: "newsecurepassword123",
	}
	resp6, _, err := doRequest(ctx.app, "POST", "/api/v1/users/"+userIDStr+"/reset-password", resetReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp6.StatusCode)

	// 7. Login with new password
	loginWithNewPwd := map[string]string{
		"username": "tran_nam",
		"password": "newsecurepassword123",
	}
	resp7, data7, err := doRequest(ctx.app, "POST", "/api/v1/auth/login", loginWithNewPwd, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp7.StatusCode)
	assert.NotEmpty(t, data7["data"].(map[string]interface{})["access_token"])
}

func TestAPI_Tasks_Flow(t *testing.T) {
	ctx := setupAPITest(t)

	now := time.Now().UTC()
	due := now.Add(72 * time.Hour)

	// 1. POST /tasks - Create root task
	createTask := tasks.CreateTaskRequest{
		Title:       "Kế hoạch triển khai mạng diện rộng",
		Description: "Nội dung kế hoạch",
		Priority:    2,
		StartAt:     &now,
		DueAt:       &due,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   ctx.admin.ID,
		},
	}
	resp, data, err := doRequest(ctx.app, "POST", "/api/v1/tasks", createTask, ctx.adminToken)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Create task failed with code %d, body: %+v", resp.StatusCode, data)
	}
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	taskData := data["data"].(map[string]interface{})
	taskIDStr := taskData["id"].(string)
	assert.NotEmpty(t, taskIDStr)
	assert.Contains(t, taskData["code"], "TASK-")

	// 2. GET /tasks - List tasks
	resp2, data2, err := doRequest(ctx.app, "GET", "/api/v1/tasks?page=1&page_size=10", nil, ctx.adminToken)
	require.NoError(t, err)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("List tasks failed with %d: %+v", resp2.StatusCode, data2)
	}
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.NotEmpty(t, data2["data"].([]interface{}))

	// 3. GET /tasks/:taskId - Detail
	resp3, data3, err := doRequest(ctx.app, "GET", "/api/v1/tasks/"+taskIDStr, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	assert.Equal(t, "Kế hoạch triển khai mạng diện rộng", data3["data"].(map[string]interface{})["title"])

	// 4. POST /tasks/:taskId/children - Create child task
	childDue := now.Add(24 * time.Hour)
	createChild := tasks.CreateChildTaskRequest{
		Title:    "Khảo sát địa hình và hạ tầng",
		Priority: 2,
		Weight:   1,
		DueAt:    &childDue,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   ctx.admin.ID,
		},
	}
	resp4, data4, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+taskIDStr+"/children", createChild, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp4.StatusCode)
	childData := data4["data"].(map[string]interface{})
	childIDStr := childData["id"].(string)

	// 5. POST /tasks/:taskId/updates - Report progress on child
	progressReq := tasks.CreateTaskUpdateRequest{
		Progress: 60,
		Content:  "Đã hoàn thành 60% khảo sát",
	}
	resp5, data5, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+childIDStr+"/updates", progressReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp5.StatusCode)
	assert.Equal(t, float64(60), data5["data"].(map[string]interface{})["progress"])

	// 6. GET /tasks/:taskId/tree - View task hierarchy
	respTree, dataTree, err := doRequest(ctx.app, "GET", "/api/v1/tasks/"+taskIDStr+"/tree", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respTree.StatusCode)
	assert.NotNil(t, dataTree["data"])

	// 7. POST & GET /tasks/:taskId/comments
	commentReq := tasks.CreateCommentRequest{
		Content: "Tiến độ đang đạt theo đúng mốc bàn giao",
	}
	respComment, dataComment, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+taskIDStr+"/comments", commentReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respComment.StatusCode)
	assert.Equal(t, "Tiến độ đang đạt theo đúng mốc bàn giao", dataComment["data"].(map[string]interface{})["content"])

	respGetComments, dataGetComments, err := doRequest(ctx.app, "GET", "/api/v1/tasks/"+taskIDStr+"/comments", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respGetComments.StatusCode)
	commentsList := dataGetComments["data"].([]interface{})
	assert.Len(t, commentsList, 1)

	// 8. POST /tasks/:taskId/cancellations - Cancel child task
	cancelReq := map[string]string{
		"reason": "Yêu cầu hủy khảo sát để điều chỉnh phương án",
	}
	resp6, data6, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+childIDStr+"/cancellations", cancelReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp6.StatusCode)
	assert.Equal(t, "cancelled", data6["data"].(map[string]interface{})["status"])
}

func TestAPI_Menus_And_RBAC(t *testing.T) {
	ctx := setupAPITest(t)

	// 1. POST /permissions - Create permission
	createPermReq := rbac.CreatePermissionRequest{
		Code:        "tasks:create",
		Name:        "Tạo công việc mới",
		Module:      "Công việc",
		Description: "Cho phép tạo mới nhiệm vụ gốc và việc con",
	}
	respPermCreate, dataPermCreate, err := doRequest(ctx.app, "POST", "/api/v1/permissions", createPermReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respPermCreate.StatusCode)
	assert.Equal(t, "tasks:create", dataPermCreate["data"].(map[string]interface{})["code"])

	// 2. GET /permissions - List
	resp1, data1, err := doRequest(ctx.app, "GET", "/api/v1/permissions", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	assert.NotEmpty(t, data1["data"].([]interface{}))

	// 3. GET /permissions/options
	resp2, data2, err := doRequest(ctx.app, "GET", "/api/v1/permissions/options", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.NotEmpty(t, data2["data"].([]interface{}))

	// 4. GET /roles
	resp3, data3, err := doRequest(ctx.app, "GET", "/api/v1/roles", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	rolesList := data3["data"].([]interface{})
	assert.NotEmpty(t, rolesList)
	firstRole := rolesList[0].(map[string]interface{})
	roleIDStr := firstRole["id"].(string)

	// 5. PUT /roles/:roleId/permissions
	updateRolePerms := rbac.UpdateRolePermissionsRequest{
		PermissionCodes: []string{"tasks:create"},
	}
	respUpdateRP, _, err := doRequest(ctx.app, "PUT", "/api/v1/roles/"+roleIDStr+"/permissions", updateRolePerms, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respUpdateRP.StatusCode)

	// 6. GET /roles/:roleId/permissions
	respGetRP, dataGetRP, err := doRequest(ctx.app, "GET", "/api/v1/roles/"+roleIDStr+"/permissions", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respGetRP.StatusCode)
	rpData := dataGetRP["data"].(map[string]interface{})
	rpPerms := rpData["permissions"].([]interface{})
	assert.Len(t, rpPerms, 1)

	// 7. POST /menus - Create Menu
	createMenuReq := menus.CreateMenuRequest{
		Code:               "tasks_management",
		Title:              "Quản lý nhiệm vụ",
		Icon:               "ProjectOutlined",
		Path:               "/tasks",
		RequiredPermission: "tasks:create",
		SortOrder:          1,
	}
	respMenuCreate, dataMenuCreate, err := doRequest(ctx.app, "POST", "/api/v1/menus", createMenuReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respMenuCreate.StatusCode)
	menuData := dataMenuCreate["data"].(map[string]interface{})
	menuIDStr := menuData["id"].(string)

	// 8. GET /menus - All menus
	respMenus, dataMenus, err := doRequest(ctx.app, "GET", "/api/v1/menus", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respMenus.StatusCode)
	assert.NotEmpty(t, dataMenus["data"].([]interface{}))

	// 9. GET /menus/user-menus (System admin gets all active menus)
	resp4, data4, err := doRequest(ctx.app, "GET", "/api/v1/menus/user-menus", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
	userMenus := data4["data"].([]interface{})
	assert.NotEmpty(t, userMenus)

	// 10. PATCH /menus/:menuId - Update menu
	newTitle := "Quản lý nhiệm vụ & Công việc"
	updateMenuReq := menus.UpdateMenuRequest{
		Title: &newTitle,
	}
	respUpdateMenu, dataUpdateMenu, err := doRequest(ctx.app, "PATCH", "/api/v1/menus/"+menuIDStr, updateMenuReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respUpdateMenu.StatusCode)
	assert.Equal(t, newTitle, dataUpdateMenu["data"].(map[string]interface{})["title"])

	// 11. DELETE /menus/:menuId
	respDeleteMenu, _, err := doRequest(ctx.app, "DELETE", "/api/v1/menus/"+menuIDStr, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDeleteMenu.StatusCode)
}

func TestAPI_Notifications(t *testing.T) {
	ctx := setupAPITest(t)

	// Seed test notification
	notif := &model.Notification{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: ctx.org.ID,
		RecipientID:    ctx.admin.ID,
		Title:          "Giao nhiệm vụ mới",
		Body:           "Bạn vừa được giao một nhiệm vụ mới",
		Type:           "task_assigned",
	}
	require.NoError(t, ctx.db.Create(notif).Error)

	// 1. GET /notifications - List
	resp1, data1, err := doRequest(ctx.app, "GET", "/api/v1/notifications?page=1&page_size=10", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	notifList := data1["data"].([]interface{})
	assert.Len(t, notifList, 1)

	// 2. GET /notifications/unread-count -> unread = 1
	resp2, data2, err := doRequest(ctx.app, "GET", "/api/v1/notifications/unread-count", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, float64(1), data2["data"].(map[string]interface{})["unread_count"])

	// 3. PATCH /notifications/:notificationId - Mark as read
	respRead, dataRead, err := doRequest(ctx.app, "PATCH", "/api/v1/notifications/"+notif.ID.String(), nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respRead.StatusCode)
	assert.NotNil(t, dataRead["data"].(map[string]interface{})["read_at"])

	// 4. GET /notifications/unread-count -> unread = 0
	respUnread2, dataUnread2, err := doRequest(ctx.app, "GET", "/api/v1/notifications/unread-count", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respUnread2.StatusCode)
	assert.Equal(t, float64(0), dataUnread2["data"].(map[string]interface{})["unread_count"])

	// 5. POST /notifications/read-all
	resp3, _, err := doRequest(ctx.app, "POST", "/api/v1/notifications/read-all", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
}

func TestAPI_Departments_And_Organizations(t *testing.T) {
	ctx := setupAPITest(t)

	// 1. GET /organizations/current
	respOrg, dataOrg, err := doRequest(ctx.app, "GET", "/api/v1/organizations/current", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respOrg.StatusCode)
	orgData := dataOrg["data"].(map[string]interface{})
	assert.Equal(t, ctx.org.Code, orgData["code"])

	// 2. POST /departments - Create Root Division
	createDiv := departments.CreateDepartmentRequest{
		Code:           "KHOI_TC",
		Name:           "Khối Tác Chiến Toàn Diện",
		DepartmentType: "division",
	}
	respDiv, dataDiv, err := doRequest(ctx.app, "POST", "/api/v1/departments", createDiv, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respDiv.StatusCode)
	divIDStr := dataDiv["data"].(map[string]interface{})["id"].(string)
	divUUID := uuid.MustParse(divIDStr)

	// 3. POST /departments - Create Child Department
	createChild := departments.CreateDepartmentRequest{
		ParentID:       &divUUID,
		Code:           "PHONG_TM",
		Name:           "Phòng Tham Mưu Chỉ Huy",
		DepartmentType: "department",
	}
	respChild, dataChild, err := doRequest(ctx.app, "POST", "/api/v1/departments", createChild, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respChild.StatusCode)
	childIDStr := dataChild["data"].(map[string]interface{})["id"].(string)

	// 4. GET /departments/tree
	respTree, dataTree, err := doRequest(ctx.app, "GET", "/api/v1/departments/tree", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respTree.StatusCode)
	treeList := dataTree["data"].([]interface{})
	assert.NotEmpty(t, treeList)

	// 5. GET /departments/options
	respOpt, dataOpt, err := doRequest(ctx.app, "GET", "/api/v1/departments/options", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respOpt.StatusCode)
	assert.GreaterOrEqual(t, len(dataOpt["data"].([]interface{})), 2)

	// 6. GET /departments/:departmentId
	respDetail, dataDetail, err := doRequest(ctx.app, "GET", "/api/v1/departments/"+childIDStr, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDetail.StatusCode)
	assert.Equal(t, "Phòng Tham Mưu Chỉ Huy", dataDetail["data"].(map[string]interface{})["name"])

	// 7. PATCH /departments/:departmentId
	newDeptName := "Phòng Tham Mưu Tác Chiến"
	patchReq := departments.UpdateDepartmentRequest{
		Name: &newDeptName,
	}
	respPatch, dataPatch, err := doRequest(ctx.app, "PATCH", "/api/v1/departments/"+childIDStr, patchReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respPatch.StatusCode)
	assert.Equal(t, newDeptName, dataPatch["data"].(map[string]interface{})["name"])

	// 8. POST /departments/:departmentId/members
	addMemberReq := departments.AddDepartmentMemberRequest{
		UserID:        ctx.admin.ID,
		PositionTitle: "Trưởng phòng",
		IsPrimary:     true,
	}
	respMember, dataMember, err := doRequest(ctx.app, "POST", "/api/v1/departments/"+childIDStr+"/members", addMemberReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respMember.StatusCode)
	memberIDStr := dataMember["data"].(map[string]interface{})["id"].(string)

	// 9. GET /departments/:departmentId/members
	respMembers, dataMembers, err := doRequest(ctx.app, "GET", "/api/v1/departments/"+childIDStr+"/members", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respMembers.StatusCode)
	assert.Len(t, dataMembers["data"].([]interface{}), 1)

	// 10. PATCH /departments/:departmentId/members/:memberId
	newPosition := "Chỉ huy trưởng phòng ban"
	patchMemberReq := departments.UpdateDepartmentMemberRequest{
		PositionTitle: &newPosition,
	}
	respPatchMember, dataPatchMember, err := doRequest(ctx.app, "PATCH", "/api/v1/departments/"+childIDStr+"/members/"+memberIDStr, patchMemberReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respPatchMember.StatusCode)
	assert.Equal(t, newPosition, dataPatchMember["data"].(map[string]interface{})["position_title"])

	// 11. DELETE /departments/:departmentId/members/:memberId
	respDelMember, _, err := doRequest(ctx.app, "DELETE", "/api/v1/departments/"+childIDStr+"/members/"+memberIDStr, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDelMember.StatusCode)

	// 12. DELETE /departments/:departmentId
	respDelDept, _, err := doRequest(ctx.app, "DELETE", "/api/v1/departments/"+childIDStr, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDelDept.StatusCode)
}

func TestAPI_Dashboard_And_Reports(t *testing.T) {
	ctx := setupAPITest(t)

	// Create a task
	now := time.Now().UTC()
	due := now.Add(48 * time.Hour)
	createTask := tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ kiểm thử Dashboard",
		Priority: 1,
		StartAt:  &now,
		DueAt:    &due,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   ctx.admin.ID,
		},
	}
	respTask, _, err := doRequest(ctx.app, "POST", "/api/v1/tasks", createTask, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respTask.StatusCode)

	// 1. GET /dashboard/summary
	respSumm, dataSumm, err := doRequest(ctx.app, "GET", "/api/v1/dashboard/summary", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respSumm.StatusCode)
	summ := dataSumm["data"].(map[string]interface{})
	assert.GreaterOrEqual(t, summ["total_tasks"].(float64), float64(1))

	// 2. GET /dashboard/my-tasks
	respMy, dataMy, err := doRequest(ctx.app, "GET", "/api/v1/dashboard/my-tasks?limit=5", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respMy.StatusCode)
	myList := dataMy["data"].([]interface{})
	assert.NotEmpty(t, myList)

	// 3. GET /reports/tasks
	respRep, dataRep, err := doRequest(ctx.app, "GET", "/api/v1/reports/tasks", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respRep.StatusCode)
	rep := dataRep["data"].(map[string]interface{})
	assert.GreaterOrEqual(t, rep["total_tasks"].(float64), float64(1))

	// 4. GET /reports/departments
	respPerf, dataPerf, err := doRequest(ctx.app, "GET", "/api/v1/reports/departments", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respPerf.StatusCode)
	assert.NotNil(t, dataPerf["data"])
}

func TestAPI_Task_AdvancedLifecycle_Attachments_Kanban(t *testing.T) {
	ctx := setupAPITest(t)

	// 1. POST /tasks with is_draft = true
	createDraft := tasks.CreateTaskRequest{
		Title:    "Kế hoạch quý 1 (Bản nháp)",
		Priority: 1,
		IsDraft:  true,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   ctx.admin.ID,
		},
	}
	respDraft, dataDraft, err := doRequest(ctx.app, "POST", "/api/v1/tasks", createDraft, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respDraft.StatusCode)
	draftID := dataDraft["data"].(map[string]interface{})["id"].(string)
	assert.Equal(t, "draft", dataDraft["data"].(map[string]interface{})["status"])

	// 2. DELETE /tasks/:taskId on active/dispatched task must be rejected
	createActive := tasks.CreateTaskRequest{
		Title:    "Công việc đã giao",
		Priority: 2,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   ctx.admin.ID,
		},
	}
	respActive, dataActive, err := doRequest(ctx.app, "POST", "/api/v1/tasks", createActive, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respActive.StatusCode)
	activeID := dataActive["data"].(map[string]interface{})["id"].(string)

	respDelActive, _, err := doRequest(ctx.app, "DELETE", "/api/v1/tasks/"+activeID, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, respDelActive.StatusCode)

	// 3. DELETE /tasks/:taskId on draft task succeeds
	respDelDraft, _, err := doRequest(ctx.app, "DELETE", "/api/v1/tasks/"+draftID, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDelDraft.StatusCode)

	// 4. Create another draft and publish it
	createDraft2 := tasks.CreateTaskRequest{
		Title:    "Bản nháp số 2",
		Priority: 2,
		IsDraft:  true,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   ctx.admin.ID,
		},
	}
	respDraft2, dataDraft2, err := doRequest(ctx.app, "POST", "/api/v1/tasks", createDraft2, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respDraft2.StatusCode)
	draftID2 := dataDraft2["data"].(map[string]interface{})["id"].(string)

	respPub, dataPub, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+draftID2+"/publish", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respPub.StatusCode)
	assert.Equal(t, "assigned", dataPub["data"].(map[string]interface{})["status"])
	assert.True(t, dataPub["data"].(map[string]interface{})["is_assigned"].(bool))

	// 5. Create unassigned task & Claim
	createUnassigned := tasks.CreateTaskRequest{
		Title:    "Công việc trong hàng đợi nhận việc",
		Priority: 3,
	}
	respUn, dataUn, err := doRequest(ctx.app, "POST", "/api/v1/tasks", createUnassigned, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respUn.StatusCode)
	unassignedID := dataUn["data"].(map[string]interface{})["id"].(string)
	assert.Equal(t, "unassigned", dataUn["data"].(map[string]interface{})["status"])
	assert.False(t, dataUn["data"].(map[string]interface{})["is_assigned"].(bool))

	// Filter by is_assigned=false
	respListUn, dataListUn, err := doRequest(ctx.app, "GET", "/api/v1/tasks?is_assigned=false", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respListUn.StatusCode)
	assert.NotEmpty(t, dataListUn["data"].([]interface{}))

	// Claim task
	respClaim, dataClaim, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+unassignedID+"/claim", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respClaim.StatusCode)
	assert.Equal(t, "assigned", dataClaim["data"].(map[string]interface{})["status"])
	assert.True(t, dataClaim["data"].(map[string]interface{})["is_assigned"].(bool))

	// 6. Block & Unblock task
	blockReq := map[string]string{"reason": "Đang chờ thiết bị từ nhà cung cấp"}
	respBlock, dataBlock, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+unassignedID+"/block", blockReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respBlock.StatusCode)
	assert.Equal(t, "blocked", dataBlock["data"].(map[string]interface{})["status"])
	assert.Equal(t, "Đang chờ thiết bị từ nhà cung cấp", dataBlock["data"].(map[string]interface{})["blocked_reason"])

	unblockReq := map[string]string{"note": "Thiết bị đã về kho"}
	respUnblock, dataUnblock, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+unassignedID+"/unblock", unblockReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respUnblock.StatusCode)
	assert.Equal(t, "in_progress", dataUnblock["data"].(map[string]interface{})["status"])

	// 7. Submit task for approval
	respSubmit, dataSubmit, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+unassignedID+"/submit", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respSubmit.StatusCode)
	assert.Equal(t, "submitted", dataSubmit["data"].(map[string]interface{})["status"])

	// 8. Move task (Kanban drag & drop)
	version := int64(dataSubmit["data"].(map[string]interface{})["version"].(float64))
	moveReq := map[string]interface{}{
		"sort_order": 10,
		"new_status": "in_progress",
		"version":    version,
	}
	respMove, dataMove, err := doRequest(ctx.app, "PATCH", "/api/v1/tasks/"+unassignedID+"/move", moveReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respMove.StatusCode)
	assert.Equal(t, "in_progress", dataMove["data"].(map[string]interface{})["status"])
	assert.Equal(t, float64(10), dataMove["data"].(map[string]interface{})["sort_order"])

	// 9. Presign attachment upload (Step 1)
	presignReq := tasks.PresignUploadRequest{
		FileName:    "spec.pdf",
		ContentType: "application/pdf",
		SizeBytes:   1024,
	}
	respPresign, dataPresign, err := doRequest(ctx.app, "POST", "/api/v1/tasks/"+unassignedID+"/attachments/presign", presignReq, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respPresign.StatusCode)
	assert.Equal(t, "local", dataPresign["data"].(map[string]interface{})["storage_driver"])

	// 10. Direct multipart upload (Step 2 or Direct local)
	fileBytes := []byte("Tài liệu đặc tả kỹ thuật chi tiết")
	respUpload, dataUpload, err := doMultipartRequest(ctx.app, "/api/v1/tasks/"+unassignedID+"/attachments/upload", "file", "spec.pdf", fileBytes, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, respUpload.StatusCode)
	attData := dataUpload["data"].(map[string]interface{})
	attID := attData["id"].(string)
	assert.Equal(t, "spec.pdf", attData["file_name"])

	// 11. List attachments
	respAttList, dataAttList, err := doRequest(ctx.app, "GET", "/api/v1/tasks/"+unassignedID+"/attachments", nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respAttList.StatusCode)
	attachments := dataAttList["data"].([]interface{})
	assert.GreaterOrEqual(t, len(attachments), 1)

	// 12. Download attachment
	reqDl := httptest.NewRequest("GET", "/api/v1/tasks/"+unassignedID+"/attachments/"+attID+"/download", nil)
	reqDl.Header.Set("Authorization", "Bearer "+ctx.adminToken)
	respDl, err := ctx.app.Test(reqDl, 10000)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDl.StatusCode)
	dlBody, err := io.ReadAll(respDl.Body)
	require.NoError(t, err)
	assert.Equal(t, fileBytes, dlBody)

	// 13. Delete attachment
	respDelAtt, _, err := doRequest(ctx.app, "DELETE", "/api/v1/tasks/"+unassignedID+"/attachments/"+attID, nil, ctx.adminToken)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respDelAtt.StatusCode)
}
