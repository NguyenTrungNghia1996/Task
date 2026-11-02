package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"taskmgmt/internal/config"
	"taskmgmt/internal/model"
	platformAuth "taskmgmt/internal/platform/auth"
)

type Service struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewService(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

func (s *Service) Login(username, password string) (*LoginResponse, error) {
	var user model.User
	if err := s.db.Where("username = ? AND is_active = ?", username, true).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	if !platformAuth.CheckPassword(user.PasswordHash, password) {
		return nil, errors.New("invalid username or password")
	}

	// Fetch roles
	var userRoles []model.UserRole
	_ = s.db.Preload("Role").Where("user_id = ?", user.ID).Find(&userRoles).Error
	roles := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		if ur.Role != nil {
			roles = append(roles, ur.Role.Code)
		}
	}

	accessToken, exp, err := platformAuth.GenerateAccessToken(
		s.cfg.JWTSecret, user.ID, user.OrganizationID, user.Username, roles, s.cfg.JWTAccessTTLMin,
	)
	if err != nil {
		return nil, err
	}

	rawRefresh, hashRefresh, err := platformAuth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshModel := model.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		ExpiresAt: time.Now().AddDate(0, 0, s.cfg.JWTRefreshTTLDays),
	}
	if err := s.db.Create(&refreshModel).Error; err != nil {
		return nil, err
	}

	perms := s.getUserPermissions(roles)

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    exp,
		User: UserInfoResp{
			ID:             user.ID,
			OrganizationID: user.OrganizationID,
			Username:       user.Username,
			FullName:       user.FullName,
			Email:          user.Email,
			Roles:          roles,
			Permissions:    perms,
		},
	}, nil
}

func (s *Service) Refresh(rawRefreshToken string) (*LoginResponse, error) {
	hash := platformAuth.HashRefreshToken(rawRefreshToken)

	var tokenRecord model.RefreshToken
	if err := s.db.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, time.Now().UTC()).
		First(&tokenRecord).Error; err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	var user model.User
	if err := s.db.Where("id = ? AND is_active = ?", tokenRecord.UserID, true).First(&user).Error; err != nil {
		return nil, errors.New("user not found or inactive")
	}

	var userRoles []model.UserRole
	_ = s.db.Preload("Role").Where("user_id = ?", user.ID).Find(&userRoles).Error
	roles := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		if ur.Role != nil {
			roles = append(roles, ur.Role.Code)
		}
	}

	newAccessToken, exp, err := platformAuth.GenerateAccessToken(
		s.cfg.JWTSecret, user.ID, user.OrganizationID, user.Username, roles, s.cfg.JWTAccessTTLMin,
	)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token (rotation)
	now := time.Now().UTC()
	tokenRecord.RevokedAt = &now
	_ = s.db.Save(&tokenRecord)

	// Issue new refresh token
	newRawRefresh, newHashRefresh, err := platformAuth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	newRefreshModel := model.RefreshToken{
		UserID:    user.ID,
		TokenHash: newHashRefresh,
		ExpiresAt: time.Now().AddDate(0, 0, s.cfg.JWTRefreshTTLDays),
	}
	_ = s.db.Create(&newRefreshModel)

	perms := s.getUserPermissions(roles)

	return &LoginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRawRefresh,
		ExpiresAt:    exp,
		User: UserInfoResp{
			ID:             user.ID,
			OrganizationID: user.OrganizationID,
			Username:       user.Username,
			FullName:       user.FullName,
			Email:          user.Email,
			Roles:          roles,
			Permissions:    perms,
		},
	}, nil
}

func (s *Service) Logout(rawRefreshToken string) error {
	hash := platformAuth.HashRefreshToken(rawRefreshToken)
	now := time.Now().UTC()
	return s.db.Model(&model.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Update("revoked_at", now).Error
}

func (s *Service) GetMe(userID uuid.UUID) (*UserInfoResp, error) {
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	var userRoles []model.UserRole
	_ = s.db.Preload("Role").Where("user_id = ?", user.ID).Find(&userRoles).Error
	roles := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		if ur.Role != nil {
			roles = append(roles, ur.Role.Code)
		}
	}

	perms := s.getUserPermissions(roles)

	return &UserInfoResp{
		ID:             user.ID,
		OrganizationID: user.OrganizationID,
		Username:       user.Username,
		FullName:       user.FullName,
		Email:          user.Email,
		Roles:          roles,
		Permissions:    perms,
	}, nil
}

func (s *Service) getUserPermissions(roles []string) []string {
	for _, r := range roles {
		if r == "system_admin" {
			var allPerms []model.Permission
			_ = s.db.Find(&allPerms).Error
			res := make([]string, len(allPerms))
			for i, p := range allPerms {
				res[i] = p.Code
			}
			return res
		}
	}

	var perms []string
	_ = s.db.Table("role_permissions").
		Joins("JOIN roles ON roles.id = role_permissions.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("roles.code IN ?", roles).
		Pluck("DISTINCT permissions.code", &perms).Error
	if perms == nil {
		perms = []string{}
	}
	return perms
}

