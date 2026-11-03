package departments

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"taskmgmt/internal/model"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetCurrentOrganization(orgID uuid.UUID) (*model.Organization, error) {
	return s.repo.GetOrganizationByID(orgID)
}

func (s *Service) GetDepartmentTree(orgID uuid.UUID) ([]DepartmentTreeNode, error) {
	depts, err := s.repo.GetDepartmentsByOrg(orgID)
	if err != nil {
		return nil, err
	}

	memberCounts, err := s.repo.GetActiveMembersCountMap(orgID)
	if err != nil {
		return nil, err
	}

	leaders, err := s.repo.GetLeadersMap(orgID)
	if err != nil {
		return nil, err
	}

	childrenMap := make(map[uuid.UUID][]model.Department)
	var rootDepts []model.Department
	deptIDMap := make(map[uuid.UUID]bool, len(depts))

	for _, d := range depts {
		deptIDMap[d.ID] = true
	}

	for _, d := range depts {
		if d.ParentID == nil || !deptIDMap[*d.ParentID] {
			rootDepts = append(rootDepts, d)
		} else {
			childrenMap[*d.ParentID] = append(childrenMap[*d.ParentID], d)
		}
	}

	var buildTree func(parentID uuid.UUID) []DepartmentTreeNode
	buildTree = func(parentID uuid.UUID) []DepartmentTreeNode {
		children := childrenMap[parentID]
		if len(children) == 0 {
			return []DepartmentTreeNode{}
		}

		nodes := make([]DepartmentTreeNode, 0, len(children))
		for _, child := range children {
			subChildren := buildTree(child.ID)
			node := DepartmentTreeNode{
				ID:             child.ID,
				Key:            child.ID.String(),
				Title:          child.Name,
				Value:          child.ID.String(),
				ParentID:       child.ParentID,
				Code:           child.Code,
				Name:           child.Name,
				DepartmentType: child.DepartmentType,
				IsActive:       child.IsActive,
				MemberCount:    memberCounts[child.ID],
				Leader:         leaders[child.ID],
				Children:       subChildren,
			}
			nodes = append(nodes, node)
		}
		return nodes
	}

	result := make([]DepartmentTreeNode, 0, len(rootDepts))
	for _, root := range rootDepts {
		subChildren := buildTree(root.ID)
		node := DepartmentTreeNode{
			ID:             root.ID,
			Key:            root.ID.String(),
			Title:          root.Name,
			Value:          root.ID.String(),
			ParentID:       root.ParentID,
			Code:           root.Code,
			Name:           root.Name,
			DepartmentType: root.DepartmentType,
			IsActive:       root.IsActive,
			MemberCount:    memberCounts[root.ID],
			Leader:         leaders[root.ID],
			Children:       subChildren,
		}
		result = append(result, node)
	}

	return result, nil
}

func (s *Service) GetDepartmentOptions(orgID uuid.UUID) ([]DepartmentOption, error) {
	depts, err := s.repo.GetDepartmentsByOrg(orgID)
	if err != nil {
		return nil, err
	}

	options := make([]DepartmentOption, 0, len(depts))
	for _, d := range depts {
		if d.IsActive {
			options = append(options, DepartmentOption{
				Value:          d.ID.String(),
				Label:          d.Name + " (" + d.Code + ")",
				Code:           d.Code,
				DepartmentType: d.DepartmentType,
				ParentID:       d.ParentID,
			})
		}
	}

	return options, nil
}

func (s *Service) GetDepartmentDetail(orgID, deptID uuid.UUID) (*DepartmentDetailResponse, error) {
	dept, err := s.repo.GetDepartmentByID(orgID, deptID)
	if err != nil {
		return nil, err
	}

	count, err := s.repo.CountActiveMembers(deptID)
	if err != nil {
		return nil, err
	}

	leaders, _ := s.repo.GetLeadersMap(orgID)

	var parentName string
	if dept.ParentID != nil {
		if parent, err := s.repo.GetDepartmentByID(orgID, *dept.ParentID); err == nil {
			parentName = parent.Name
		}
	}

	return &DepartmentDetailResponse{
		ID:             dept.ID,
		OrganizationID: dept.OrganizationID,
		ParentID:       dept.ParentID,
		ParentName:     parentName,
		Code:           dept.Code,
		Name:           dept.Name,
		DepartmentType: dept.DepartmentType,
		IsActive:       dept.IsActive,
		MemberCount:    int(count),
		Leader:         leaders[dept.ID],
		CreatedAt:      dept.CreatedAt,
		UpdatedAt:      dept.UpdatedAt,
	}, nil
}

func (s *Service) CreateDepartment(orgID uuid.UUID, req CreateDepartmentRequest) (*DepartmentDetailResponse, error) {
	existing, err := s.repo.GetDepartmentByCode(orgID, req.Code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDepartmentCodeExists
	}

	if req.ParentID != nil {
		_, err := s.repo.GetDepartmentByID(orgID, *req.ParentID)
		if err != nil {
			return nil, ErrParentDepartmentNotFound
		}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	dept := &model.Department{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		ParentID:       req.ParentID,
		Code:           req.Code,
		Name:           req.Name,
		DepartmentType: req.DepartmentType,
		IsActive:       isActive,
	}

	if err := s.repo.CreateDepartment(dept); err != nil {
		return nil, err
	}

	return s.GetDepartmentDetail(orgID, dept.ID)
}

func (s *Service) UpdateDepartment(orgID, deptID uuid.UUID, req UpdateDepartmentRequest) (*DepartmentDetailResponse, error) {
	dept, err := s.repo.GetDepartmentByID(orgID, deptID)
	if err != nil {
		return nil, err
	}

	if req.Code != nil && *req.Code != dept.Code {
		existing, err := s.repo.GetDepartmentByCode(orgID, *req.Code)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != deptID {
			return nil, ErrDepartmentCodeExists
		}
		dept.Code = *req.Code
	}

	if req.ParentID != nil {
		if dept.ParentID == nil || *req.ParentID != *dept.ParentID {
			// Check parent exists
			_, err := s.repo.GetDepartmentByID(orgID, *req.ParentID)
			if err != nil {
				return nil, ErrParentDepartmentNotFound
			}

			// Check cycle
			isCycle, err := s.repo.CheckDepartmentCycle(deptID, *req.ParentID)
			if err != nil {
				return nil, err
			}
			if isCycle {
				return nil, ErrDepartmentCycleDetected
			}
			dept.ParentID = req.ParentID
		}
	}

	if req.Name != nil {
		dept.Name = *req.Name
	}
	if req.DepartmentType != nil {
		dept.DepartmentType = *req.DepartmentType
	}
	if req.IsActive != nil {
		dept.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateDepartment(dept); err != nil {
		return nil, err
	}

	return s.GetDepartmentDetail(orgID, dept.ID)
}

func (s *Service) DeleteDepartment(orgID, deptID uuid.UUID) error {
	_, err := s.repo.GetDepartmentByID(orgID, deptID)
	if err != nil {
		return err
	}

	hasChildren, err := s.repo.HasChildDepartments(deptID)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrDepartmentHasChildren
	}

	memberCount, err := s.repo.CountActiveMembers(deptID)
	if err != nil {
		return err
	}
	if memberCount > 0 {
		return ErrDepartmentHasMembers
	}

	return s.repo.DeleteDepartment(orgID, deptID)
}

func (s *Service) GetDepartmentMembers(orgID, deptID uuid.UUID) ([]DepartmentMemberDTO, error) {
	if _, err := s.repo.GetDepartmentByID(orgID, deptID); err != nil {
		return nil, err
	}
	return s.repo.GetDepartmentMembers(deptID)
}

func (s *Service) AddDepartmentMember(orgID, deptID uuid.UUID, req AddDepartmentMemberRequest) (*DepartmentMemberDTO, error) {
	if _, err := s.repo.GetDepartmentByID(orgID, deptID); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(orgID, req.UserID)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetMemberByUserAndDept(deptID, req.UserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrMemberAlreadyExists
	}

	if req.ManagerUserID != nil {
		if _, err := s.repo.GetUserByID(orgID, *req.ManagerUserID); err != nil {
			return nil, errors.New("manager user not found in organization")
		}
	}

	effFrom := time.Now().UTC()
	if req.EffectiveFrom != nil {
		effFrom = *req.EffectiveFrom
	}

	member := &model.DepartmentMember{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		DepartmentID:   deptID,
		UserID:         req.UserID,
		ManagerUserID:  req.ManagerUserID,
		PositionTitle:  req.PositionTitle,
		IsPrimary:      req.IsPrimary,
		EffectiveFrom:  effFrom,
		EffectiveTo:    req.EffectiveTo,
	}

	if err := s.repo.AddDepartmentMember(member); err != nil {
		return nil, err
	}

	dto := &DepartmentMemberDTO{
		ID:            member.ID,
		DepartmentID:  member.DepartmentID,
		UserID:        user.ID,
		Username:      user.Username,
		FullName:      user.FullName,
		Email:         user.Email,
		ManagerUserID: req.ManagerUserID,
		PositionTitle: member.PositionTitle,
		IsPrimary:     member.IsPrimary,
		EffectiveFrom: member.EffectiveFrom,
		EffectiveTo:   member.EffectiveTo,
	}

	if req.ManagerUserID != nil {
		if mgr, err := s.repo.GetUserByID(orgID, *req.ManagerUserID); err == nil {
			dto.ManagerName = mgr.FullName
		}
	}

	return dto, nil
}

func (s *Service) UpdateDepartmentMember(orgID, deptID, memberID uuid.UUID, req UpdateDepartmentMemberRequest) (*DepartmentMemberDTO, error) {
	if _, err := s.repo.GetDepartmentByID(orgID, deptID); err != nil {
		return nil, err
	}

	member, err := s.repo.GetMemberByID(deptID, memberID)
	if err != nil {
		return nil, err
	}

	if req.PositionTitle != nil {
		member.PositionTitle = *req.PositionTitle
	}
	if req.IsPrimary != nil {
		member.IsPrimary = *req.IsPrimary
	}
	if req.EffectiveFrom != nil {
		member.EffectiveFrom = *req.EffectiveFrom
	}
	if req.EffectiveTo != nil {
		member.EffectiveTo = req.EffectiveTo
	}
	if req.ManagerUserID != nil {
		if _, err := s.repo.GetUserByID(orgID, *req.ManagerUserID); err != nil {
			return nil, errors.New("manager user not found in organization")
		}
		member.ManagerUserID = req.ManagerUserID
	}

	if err := s.repo.UpdateDepartmentMember(member); err != nil {
		return nil, err
	}

	user, _ := s.repo.GetUserByID(orgID, member.UserID)
	dto := &DepartmentMemberDTO{
		ID:            member.ID,
		DepartmentID:  member.DepartmentID,
		UserID:        member.UserID,
		ManagerUserID: member.ManagerUserID,
		PositionTitle: member.PositionTitle,
		IsPrimary:     member.IsPrimary,
		EffectiveFrom: member.EffectiveFrom,
		EffectiveTo:   member.EffectiveTo,
	}
	if user != nil {
		dto.Username = user.Username
		dto.FullName = user.FullName
		dto.Email = user.Email
	}
	if member.ManagerUserID != nil {
		if mgr, err := s.repo.GetUserByID(orgID, *member.ManagerUserID); err == nil {
			dto.ManagerName = mgr.FullName
		}
	}

	return dto, nil
}

func (s *Service) RemoveDepartmentMember(orgID, deptID, memberID uuid.UUID) error {
	if _, err := s.repo.GetDepartmentByID(orgID, deptID); err != nil {
		return err
	}

	if _, err := s.repo.GetMemberByID(deptID, memberID); err != nil {
		return err
	}

	return s.repo.RemoveDepartmentMember(deptID, memberID)
}
