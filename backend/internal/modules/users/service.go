package users

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"taskmgmt/internal/model"
	platformAuth "taskmgmt/internal/platform/auth"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListUsers(orgID uuid.UUID, filter UserFilter) ([]UserListItem, int64, error) {
	return s.repo.ListUsers(orgID, filter)
}

func (s *Service) GetUserDetail(orgID, userID uuid.UUID) (*UserDetailResponse, error) {
	user, err := s.repo.GetUserByID(orgID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("người dùng không tồn tại")
		}
		return nil, err
	}
	return user, nil
}

func (s *Service) CreateUser(orgID uuid.UUID, req CreateUserRequest) (*UserDetailResponse, error) {
	// Kiểm tra username đã tồn tại chưa
	existing, err := s.repo.FindByUsername(orgID, req.Username)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("tên đăng nhập đã tồn tại trong đơn vị")
	}

	hash, err := platformAuth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		BaseModel:      model.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		OrganizationID: orgID,
		Username:       req.Username,
		FullName:       req.FullName,
		Email:          req.Email,
		PasswordHash:   hash,
		IsActive:       true,
	}

	if err := s.repo.CreateUser(orgID, user, req.RoleCodes, req.DepartmentID, req.PositionTitle); err != nil {
		return nil, err
	}

	return s.GetUserDetail(orgID, user.ID)
}

func (s *Service) UpdateUser(orgID, userID uuid.UUID, req UpdateUserRequest) (*UserDetailResponse, error) {
	if _, err := s.GetUserDetail(orgID, userID); err != nil {
		return nil, err
	}

	if err := s.repo.UpdateUser(orgID, userID, req); err != nil {
		return nil, err
	}

	return s.GetUserDetail(orgID, userID)
}

func (s *Service) SetUserRoles(orgID, userID uuid.UUID, roleCodes []string) (*UserDetailResponse, error) {
	if _, err := s.GetUserDetail(orgID, userID); err != nil {
		return nil, err
	}

	if err := s.repo.SetUserRoles(orgID, userID, roleCodes); err != nil {
		return nil, err
	}

	return s.GetUserDetail(orgID, userID)
}

func (s *Service) ResetPassword(orgID, userID uuid.UUID, newPassword string) error {
	if _, err := s.GetUserDetail(orgID, userID); err != nil {
		return err
	}

	hash, err := platformAuth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.repo.ResetPassword(orgID, userID, hash)
}

func (s *Service) GetUserOptions(orgID uuid.UUID) ([]UserOption, error) {
	return s.repo.GetUserOptions(orgID)
}
