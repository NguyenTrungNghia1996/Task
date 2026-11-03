package departments_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/departments"
	"taskmgmt/internal/testutil"
)

func TestDepartment_Tree_And_CRUD(t *testing.T) {
	db := testutil.SetupTestDB(t)
	org, admin := testutil.CreateTestOrgAndUser(t, db, "admin_user", "system_admin")

	repo := departments.NewRepository(db)
	svc := departments.NewService(repo)

	// 1. GetCurrentOrganization
	currentOrg, err := svc.GetCurrentOrganization(org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.Code, currentOrg.Code)

	// 2. Create Root Department: Khối Tác Chiến
	rootReq := departments.CreateDepartmentRequest{
		Code:           "KTC",
		Name:           "Khối Tác Chiến",
		DepartmentType: "division",
	}
	rootDept, err := svc.CreateDepartment(org.ID, rootReq)
	require.NoError(t, err)
	assert.NotEmpty(t, rootDept.ID)
	assert.Equal(t, "KTC", rootDept.Code)
	assert.Equal(t, "Khối Tác Chiến", rootDept.Name)

	// 3. Prevent duplicate code
	_, err = svc.CreateDepartment(org.ID, rootReq)
	assert.ErrorIs(t, err, departments.ErrDepartmentCodeExists)

	// 4. Create Child Department: Phòng Tham mưu
	childReq := departments.CreateDepartmentRequest{
		ParentID:       &rootDept.ID,
		Code:           "PTM",
		Name:           "Phòng Tham Mưu",
		DepartmentType: "department",
	}
	childDept, err := svc.CreateDepartment(org.ID, childReq)
	require.NoError(t, err)
	assert.Equal(t, &rootDept.ID, childDept.ParentID)
	assert.Equal(t, "Khối Tác Chiến", childDept.ParentName)

	// 5. Create Grandchild Department: Ban Thông Tin
	grandChildReq := departments.CreateDepartmentRequest{
		ParentID:       &childDept.ID,
		Code:           "BTT",
		Name:           "Ban Thông Tin",
		DepartmentType: "section",
	}
	grandChildDept, err := svc.CreateDepartment(org.ID, grandChildReq)
	require.NoError(t, err)
	assert.Equal(t, &childDept.ID, grandChildDept.ParentID)

	// 6. Test Tree structure
	tree, err := svc.GetDepartmentTree(org.ID)
	require.NoError(t, err)
	require.Len(t, tree, 1) // Only 1 root dept
	assert.Equal(t, "KTC", tree[0].Code)
	assert.Equal(t, rootDept.ID.String(), tree[0].Key)
	require.Len(t, tree[0].Children, 1) // PTM
	assert.Equal(t, "PTM", tree[0].Children[0].Code)
	require.Len(t, tree[0].Children[0].Children, 1) // BTT
	assert.Equal(t, "BTT", tree[0].Children[0].Children[0].Code)

	// 7. Test Options (Select format)
	options, err := svc.GetDepartmentOptions(org.ID)
	require.NoError(t, err)
	assert.Len(t, options, 3)

	// 8. Test Cycle Prevention: Attempt to make rootDept child of grandChildDept
	updateCycleReq := departments.UpdateDepartmentRequest{
		ParentID: &grandChildDept.ID,
	}
	_, err = svc.UpdateDepartment(org.ID, rootDept.ID, updateCycleReq)
	assert.ErrorIs(t, err, departments.ErrDepartmentCycleDetected)

	// 9. Update Department Info
	newName := "Khối Tác Chiến & Chiến lược"
	updateReq := departments.UpdateDepartmentRequest{
		Name: &newName,
	}
	updated, err := svc.UpdateDepartment(org.ID, rootDept.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)

	// 10. Delete Department Constraint Checks
	// Root has children -> Cannot delete
	err = svc.DeleteDepartment(org.ID, rootDept.ID)
	assert.ErrorIs(t, err, departments.ErrDepartmentHasChildren)

	// Add member to grandChildDept
	addMemberReq := departments.AddDepartmentMemberRequest{
		UserID:        admin.ID,
		PositionTitle: "Trưởng ban",
		IsPrimary:     true,
	}
	member, err := svc.AddDepartmentMember(org.ID, grandChildDept.ID, addMemberReq)
	require.NoError(t, err)
	assert.Equal(t, "Trưởng ban", member.PositionTitle)
	assert.Equal(t, "admin_user", member.Username)

	// GrandChild has member -> Cannot delete
	err = svc.DeleteDepartment(org.ID, grandChildDept.ID)
	assert.ErrorIs(t, err, departments.ErrDepartmentHasMembers)

	// Remove member -> Now can delete grandChild
	err = svc.RemoveDepartmentMember(org.ID, grandChildDept.ID, member.ID)
	require.NoError(t, err)

	err = svc.DeleteDepartment(org.ID, grandChildDept.ID)
	require.NoError(t, err)
}

func TestDepartment_Members(t *testing.T) {
	db := testutil.SetupTestDB(t)
	org, admin := testutil.CreateTestOrgAndUser(t, db, "admin_user", "system_admin")

	// Create user 2
	user2 := &model.User{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Username:       "staff_one",
		FullName:       "Nguyễn Văn Staff",
		Email:          "staff@test.com",
		IsActive:       true,
	}
	require.NoError(t, db.Create(user2).Error)

	repo := departments.NewRepository(db)
	svc := departments.NewService(repo)

	// Create Department
	dept, err := svc.CreateDepartment(org.ID, departments.CreateDepartmentRequest{
		Code:           "PKH",
		Name:           "Phòng Kế Hoạch",
		DepartmentType: "department",
	})
	require.NoError(t, err)

	// 1. Add Leader
	leaderReq := departments.AddDepartmentMemberRequest{
		UserID:        admin.ID,
		PositionTitle: "Trưởng phòng",
		IsPrimary:     true,
	}
	leader, err := svc.AddDepartmentMember(org.ID, dept.ID, leaderReq)
	require.NoError(t, err)
	assert.Equal(t, admin.ID, leader.UserID)

	// 2. Add Staff with manager
	staffReq := departments.AddDepartmentMemberRequest{
		UserID:        user2.ID,
		PositionTitle: "Chuyên viên",
		ManagerUserID: &admin.ID,
		IsPrimary:     true,
	}
	staff, err := svc.AddDepartmentMember(org.ID, dept.ID, staffReq)
	require.NoError(t, err)
	assert.Equal(t, &admin.ID, staff.ManagerUserID)
	assert.Equal(t, admin.FullName, staff.ManagerName)

	// 3. Prevent duplicate member
	_, err = svc.AddDepartmentMember(org.ID, dept.ID, staffReq)
	assert.ErrorIs(t, err, departments.ErrMemberAlreadyExists)

	// 4. List members
	members, err := svc.GetDepartmentMembers(org.ID, dept.ID)
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// 5. Update member position
	newPos := "Phó Trưởng phòng"
	updatedStaff, err := svc.UpdateDepartmentMember(org.ID, dept.ID, staff.ID, departments.UpdateDepartmentMemberRequest{
		PositionTitle: &newPos,
	})
	require.NoError(t, err)
	assert.Equal(t, newPos, updatedStaff.PositionTitle)

	// 6. Check Department detail shows member count and leader
	detail, err := svc.GetDepartmentDetail(org.ID, dept.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, detail.MemberCount)
	require.NotNil(t, detail.Leader)
	assert.Equal(t, admin.ID, detail.Leader.UserID)
}
