package testutil

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"taskmgmt/internal/model"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:?_pragma=foreign_keys(1)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&model.Organization{},
		&model.Department{},
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.DepartmentMember{},
		&model.Permission{},
		&model.RolePermission{},
		&model.Menu{},
		&model.Task{},
		&model.TaskAssignment{},
		&model.TaskUpdate{},
		&model.TaskReview{},
		&model.TaskComment{},
		&model.TaskAttachment{},
		&model.TaskEvent{},
		&model.RefreshToken{},
		&model.Notification{},
	)
	require.NoError(t, err)

	return db
}

func CreateTestOrgAndUser(t *testing.T, db *gorm.DB, username string, roleCodes ...string) (*model.Organization, *model.User) {
	t.Helper()

	var org model.Organization
	err := db.Where("code = ?", "ORG_TEST").First(&org).Error
	if err != nil {
		org = model.Organization{
			BaseModel: model.BaseModel{ID: uuid.New()},
			Code:      "ORG_TEST",
			Name:      "Test Organization",
			IsActive:  true,
		}
		require.NoError(t, db.Create(&org).Error)
	}

	user := &model.User{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Username:       username,
		FullName:       "Test " + username,
		Email:          username + "@example.com",
		PasswordHash:   "$2a$10$F5sIz7VVLUrjRzmCb7B0wO49U9fjogpj0R1h8lIhxLea9cmG5HKU6", // password123
		IsActive:       true,
	}
	require.NoError(t, db.Create(user).Error)

	for _, rc := range roleCodes {
		role := &model.Role{
			BaseModel: model.BaseModel{ID: uuid.New()},
			Code:      rc,
			Name:      rc,
		}
		_ = db.FirstOrCreate(role, model.Role{Code: rc})

		ur := &model.UserRole{
			BaseModel:      model.BaseModel{ID: uuid.New()},
			OrganizationID: org.ID,
			UserID:         user.ID,
			RoleID:         role.ID,
		}
		require.NoError(t, db.Create(ur).Error)
	}

	return &org, user
}
