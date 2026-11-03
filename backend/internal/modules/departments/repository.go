package departments

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"taskmgmt/internal/model"
)

var (
	ErrDepartmentNotFound       = errors.New("department not found")
	ErrDepartmentCodeExists     = errors.New("department code already exists in organization")
	ErrDepartmentCycleDetected  = errors.New("cannot set self or descendant department as parent (cycle detected)")
	ErrParentDepartmentNotFound = errors.New("parent department not found in organization")
	ErrDepartmentHasChildren    = errors.New("cannot delete department with child departments")
	ErrDepartmentHasMembers     = errors.New("cannot delete department with active members")
	ErrMemberAlreadyExists      = errors.New("user is already a member of this department")
	ErrMemberNotFound           = errors.New("department member not found")
	ErrUserNotFound             = errors.New("user not found in organization")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetOrganizationByID(orgID uuid.UUID) (*model.Organization, error) {
	var org model.Organization
	if err := r.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *Repository) GetDepartmentsByOrg(orgID uuid.UUID) ([]model.Department, error) {
	var list []model.Department
	err := r.db.Where("organization_id = ?", orgID).
		Order("name ASC, code ASC").
		Find(&list).Error
	return list, err
}

func (r *Repository) GetDepartmentByID(orgID, deptID uuid.UUID) (*model.Department, error) {
	var dept model.Department
	err := r.db.Where("organization_id = ? AND id = ?", orgID, deptID).First(&dept).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDepartmentNotFound
		}
		return nil, err
	}
	return &dept, nil
}

func (r *Repository) GetDepartmentByCode(orgID uuid.UUID, code string) (*model.Department, error) {
	var dept model.Department
	err := r.db.Where("organization_id = ? AND code = ?", orgID, code).First(&dept).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &dept, nil
}

func (r *Repository) CreateDepartment(dept *model.Department) error {
	return r.db.Create(dept).Error
}

func (r *Repository) UpdateDepartment(dept *model.Department) error {
	return r.db.Save(dept).Error
}

func (r *Repository) DeleteDepartment(orgID, deptID uuid.UUID) error {
	return r.db.Where("organization_id = ? AND id = ?", orgID, deptID).Delete(&model.Department{}).Error
}

func (r *Repository) HasChildDepartments(deptID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&model.Department{}).Where("parent_id = ?", deptID).Count(&count).Error
	return count > 0, err
}

func (r *Repository) CountActiveMembers(deptID uuid.UUID) (int64, error) {
	var count int64
	now := time.Now().UTC()
	err := r.db.Model(&model.DepartmentMember{}).
		Where("department_id = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to > ?)", deptID, now, now).
		Count(&count).Error
	return count, err
}

func (r *Repository) CheckDepartmentCycle(deptID, newParentID uuid.UUID) (bool, error) {
	if deptID == newParentID {
		return true, nil
	}

	currID := &newParentID
	visited := make(map[uuid.UUID]bool)

	for currID != nil {
		if visited[*currID] {
			return true, nil // Loop detected in existing data
		}
		visited[*currID] = true

		if *currID == deptID {
			return true, nil // Cycle: newParentID is a descendant of deptID
		}

		var d model.Department
		err := r.db.Select("id", "parent_id").Where("id = ?", *currID).First(&d).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return false, err
		}
		currID = d.ParentID
	}

	return false, nil
}

func (r *Repository) GetDepartmentMembers(deptID uuid.UUID) ([]DepartmentMemberDTO, error) {
	var members []model.DepartmentMember
	err := r.db.Where("department_id = ?", deptID).
		Preload("User").
		Order("is_primary DESC, created_at ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}

	result := make([]DepartmentMemberDTO, 0, len(members))
	for _, m := range members {
		dto := DepartmentMemberDTO{
			ID:            m.ID,
			DepartmentID:  m.DepartmentID,
			UserID:        m.UserID,
			PositionTitle: m.PositionTitle,
			IsPrimary:     m.IsPrimary,
			EffectiveFrom: m.EffectiveFrom,
			EffectiveTo:   m.EffectiveTo,
		}
		if m.User != nil {
			dto.Username = m.User.Username
			dto.FullName = m.User.FullName
			dto.Email = m.User.Email
		}
		if m.ManagerUserID != nil {
			dto.ManagerUserID = m.ManagerUserID
			var manager model.User
			if err := r.db.Where("id = ?", *m.ManagerUserID).First(&manager).Error; err == nil {
				dto.ManagerName = manager.FullName
			}
		}
		result = append(result, dto)
	}

	return result, nil
}

func (r *Repository) GetMemberByID(deptID, memberID uuid.UUID) (*model.DepartmentMember, error) {
	var m model.DepartmentMember
	err := r.db.Where("department_id = ? AND id = ?", deptID, memberID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *Repository) GetMemberByUserAndDept(deptID, userID uuid.UUID) (*model.DepartmentMember, error) {
	var m model.DepartmentMember
	err := r.db.Where("department_id = ? AND user_id = ?", deptID, userID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *Repository) AddDepartmentMember(member *model.DepartmentMember) error {
	return r.db.Create(member).Error
}

func (r *Repository) UpdateDepartmentMember(member *model.DepartmentMember) error {
	return r.db.Save(member).Error
}

func (r *Repository) RemoveDepartmentMember(deptID, memberID uuid.UUID) error {
	return r.db.Where("department_id = ? AND id = ?", deptID, memberID).Delete(&model.DepartmentMember{}).Error
}

func (r *Repository) GetUserByID(orgID, userID uuid.UUID) (*model.User, error) {
	var u model.User
	err := r.db.Where("organization_id = ? AND id = ?", orgID, userID).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) GetActiveMembersCountMap(orgID uuid.UUID) (map[uuid.UUID]int, error) {
	type countResult struct {
		DepartmentID uuid.UUID
		Total        int
	}

	var results []countResult
	now := time.Now().UTC()
	err := r.db.Model(&model.DepartmentMember{}).
		Select("department_id, count(id) as total").
		Where("organization_id = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to > ?)", orgID, now, now).
		Group("department_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	countMap := make(map[uuid.UUID]int, len(results))
	for _, res := range results {
		countMap[res.DepartmentID] = res.Total
	}
	return countMap, nil
}

func (r *Repository) GetLeadersMap(orgID uuid.UUID) (map[uuid.UUID]*DepartmentLeaderDTO, error) {
	var members []model.DepartmentMember
	now := time.Now().UTC()
	// Find primary members or members with position title containing Trưởng / Chỉ huy / Leader
	err := r.db.Where("organization_id = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to > ?)", orgID, now, now).
		Preload("User").
		Order("is_primary DESC, created_at ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}

	leaderMap := make(map[uuid.UUID]*DepartmentLeaderDTO)
	for _, m := range members {
		if _, exists := leaderMap[m.DepartmentID]; !exists {
			if m.User != nil {
				leaderMap[m.DepartmentID] = &DepartmentLeaderDTO{
					UserID:        m.User.ID,
					FullName:      m.User.FullName,
					Username:      m.User.Username,
					PositionTitle: m.PositionTitle,
				}
			}
		}
	}

	return leaderMap, nil
}
