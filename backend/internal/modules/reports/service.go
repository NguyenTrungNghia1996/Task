package reports

import (
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetDashboardSummary(orgID, userID uuid.UUID, roles []string) (*DashboardSummaryResponse, error) {
	isAllScope := false
	for _, r := range roles {
		if r == "system_admin" || r == "commander" {
			isAllScope = true
			break
		}
	}

	var deptIDs []uuid.UUID
	if !isAllScope {
		var err error
		deptIDs, err = s.repo.GetUserDepartmentIDs(orgID, userID)
		if err != nil {
			return nil, err
		}
	}

	return s.repo.GetDashboardSummary(orgID, userID, isAllScope, deptIDs)
}

func (s *Service) GetMyTasks(orgID, userID uuid.UUID, limit int) ([]MyTaskItem, error) {
	deptIDs, err := s.repo.GetUserDepartmentIDs(orgID, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetMyTasks(orgID, userID, deptIDs, limit)
}

func (s *Service) GetTaskReport(orgID uuid.UUID, filter TaskReportFilter) (*TaskReportResponse, error) {
	return s.repo.GetTaskReport(orgID, filter)
}

func (s *Service) GetDepartmentPerformance(orgID uuid.UUID) ([]DepartmentPerformanceItem, error) {
	return s.repo.GetDepartmentPerformance(orgID)
}

