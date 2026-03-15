package service

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

type ActorSummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type DepartmentSummary struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	CreatedByUserID string    `json:"created_by_user_id,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	MemberCount int64         `json:"member_count"`
	CreatedBy   *ActorSummary `json:"created_by,omitempty"`
}

type RoleMemberSummary struct {
	ID         string `json:"id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	JobTitle   string `json:"job_title"`
	Department string `json:"department,omitempty"`
}

type RoleSummary struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	CreatedByUserID string          `json:"created_by_user_id,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	MemberCount int64               `json:"member_count"`
	CreatedBy   *ActorSummary       `json:"created_by,omitempty"`
	Members     []RoleMemberSummary `json:"members,omitempty"`
}

func actorSummaryFromUser(user models.User) *ActorSummary {
	name := strings.TrimSpace(user.FullName())
	if name == "" {
		name = user.Email
	}
	if name == "" {
		name = "User " + user.ID
	}
	return &ActorSummary{ID: user.ID, Name: name, Email: user.Email}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func hasUserRoleMembershipsTable(tx *gorm.DB) bool {
	return tx.Migrator().HasTable(&models.UserRoleMembership{})
}

func (s *EmployeeService) CreateRole(orgID, name, description, createdBy string, memberIDs []string) (*models.Role, error) {
	var existing models.Role
	err := s.db.Where("name = ? AND organization_id = ?", name, orgID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: role %q already exists in this organization", ErrDuplicateRole, name)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing role: %w", err)
	}

	role := models.Role{
		OrganizationID: orgID,
		Name:           name,
		Description:    description,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if createdBy != "" {
		role.CreatedByUserID = &createdBy
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("%w: role %q already exists in this organization", ErrDuplicateRole, name)
			}
			return fmt.Errorf("failed to create role: %w", err)
		}
		if err := s.addUsersToRole(tx, orgID, role.ID, createdBy, memberIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Printf("Role created: %s in org %s", role.ID, orgID)
	return &role, nil
}

func (s *EmployeeService) addUsersToRole(tx *gorm.DB, orgID, roleID, assignedBy string, memberIDs []string) error {
	memberIDs = uniqueStrings(memberIDs)
	if len(memberIDs) == 0 {
		return nil
	}

	var users []models.User
	if err := tx.Where("organization_id = ? AND id IN ?", orgID, memberIDs).Find(&users).Error; err != nil {
		return fmt.Errorf("failed to validate role members: %w", err)
	}
	if len(users) != len(memberIDs) {
		return fmt.Errorf("one or more selected users do not belong to this organization")
	}

	if !hasUserRoleMembershipsTable(tx) {
		if err := tx.Model(&models.User{}).
			Where("organization_id = ? AND id IN ?", orgID, memberIDs).
			Updates(map[string]interface{}{"role_id": roleID, "updated_at": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to assign users to role via legacy role_id: %w", err)
		}
		return nil
	}

	memberships := make([]models.UserRoleMembership, 0, len(users))
	for _, user := range users {
		membership := models.UserRoleMembership{
			OrganizationID: orgID,
			UserID:         user.ID,
			RoleID:         roleID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if assignedBy != "" {
			membership.AssignedBy = &assignedBy
		}
		memberships = append(memberships, membership)
	}

	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "organization_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).Create(&memberships).Error; err != nil {
		return fmt.Errorf("failed to assign users to role: %w", err)
	}

	return nil
}

func (s *EmployeeService) AssignRoleNamesToUser(tx *gorm.DB, orgID, userID, assignedBy string, roleNames []string) error {
	roleNames = uniqueStrings(roleNames)
	if len(roleNames) == 0 {
		return nil
	}

	var roles []models.Role
	if err := tx.Where("organization_id = ? AND name IN ?", orgID, roleNames).Find(&roles).Error; err != nil {
		return fmt.Errorf("failed to resolve invited roles: %w", err)
	}
	if len(roles) == 0 {
		return nil
	}

	if !hasUserRoleMembershipsTable(tx) {
		// Legacy schema supports only one role pointer on users.
		if err := tx.Model(&models.User{}).
			Where("organization_id = ? AND id = ?", orgID, userID).
			Updates(map[string]interface{}{"role_id": roles[0].ID, "updated_at": time.Now()}).Error; err != nil {
			return fmt.Errorf("failed to assign invited role via legacy role_id: %w", err)
		}
		return nil
	}

	memberships := make([]models.UserRoleMembership, 0, len(roles))
	for _, role := range roles {
		membership := models.UserRoleMembership{
			OrganizationID: orgID,
			UserID:         userID,
			RoleID:         role.ID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if assignedBy != "" {
			membership.AssignedBy = &assignedBy
		}
		memberships = append(memberships, membership)
	}

	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "organization_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).Create(&memberships).Error; err != nil {
		return fmt.Errorf("failed to assign invited roles: %w", err)
	}

	return nil
}

func (s *EmployeeService) ListDepartmentSummaries(orgID string) ([]DepartmentSummary, error) {
	var departments []models.Department
	if err := s.db.Where("organization_id = ?", orgID).Order("name ASC").Find(&departments).Error; err != nil {
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}

	memberCounts := make(map[string]int64, len(departments))
	for _, department := range departments {
		var count int64
		if err := s.db.Model(&models.User{}).Where("organization_id = ? AND department_id = ?", orgID, department.ID).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count department members: %w", err)
		}
		memberCounts[department.ID] = count
	}

	creatorIDs := make([]string, 0, len(departments))
	for _, department := range departments {
		if department.CreatedByUserID != nil {
			creatorIDs = append(creatorIDs, *department.CreatedByUserID)
		}
	}
	creatorIDs = uniqueStrings(creatorIDs)

	creators := make(map[string]models.User, len(creatorIDs))
	if len(creatorIDs) > 0 {
		var users []models.User
		if err := s.db.Where("id IN ?", creatorIDs).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("failed to load department creators: %w", err)
		}
		for _, user := range users {
			creators[user.ID] = user
		}
	}

	summaries := make([]DepartmentSummary, 0, len(departments))
	for _, department := range departments {
		summary := DepartmentSummary{
			ID:          department.ID,
			Name:        department.Name,
			Description: department.Description,
			CreatedByUserID: func() string {
				if department.CreatedByUserID != nil {
					return *department.CreatedByUserID
				}
				return ""
			}(),
			CreatedAt:   department.CreatedAt,
			UpdatedAt:   department.UpdatedAt,
			MemberCount: memberCounts[department.ID],
		}
		if department.CreatedByUserID != nil {
			if creator, ok := creators[*department.CreatedByUserID]; ok {
				summary.CreatedBy = actorSummaryFromUser(creator)
			} else {
				summary.CreatedBy = &ActorSummary{ID: *department.CreatedByUserID, Name: "User " + *department.CreatedByUserID}
			}
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (s *EmployeeService) ListRoleSummaries(orgID string) ([]RoleSummary, error) {
	var roles []models.Role
	if err := s.db.Where("organization_id = ?", orgID).Order("name ASC").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	membersByRole := make(map[string][]RoleMemberSummary, len(roles))

	if hasUserRoleMembershipsTable(s.db) {
		var memberships []models.UserRoleMembership
		if err := s.db.
			Where("organization_id = ?", orgID).
			Preload("User").
			Preload("User.Department").
			Find(&memberships).Error; err != nil {
			return nil, fmt.Errorf("failed to load role memberships: %w", err)
		}

		for _, membership := range memberships {
			membersByRole[membership.RoleID] = append(membersByRole[membership.RoleID], RoleMemberSummary{
				ID:         membership.User.ID,
				FirstName:  membership.User.FirstName,
				LastName:   membership.User.LastName,
				Email:      membership.User.Email,
				JobTitle:   membership.User.JobTitle,
				Department: func() string {
					if membership.User.Department != nil {
						return membership.User.Department.Name
					}
					return ""
				}(),
			})
		}
	} else {
		roleIDs := make([]string, 0, len(roles))
		for _, role := range roles {
			roleIDs = append(roleIDs, role.ID)
		}
		if len(roleIDs) > 0 {
			var users []models.User
			if err := s.db.
				Where("organization_id = ? AND role_id IN ?", orgID, roleIDs).
				Preload("Department").
				Find(&users).Error; err != nil {
				return nil, fmt.Errorf("failed to load role members via legacy role_id: %w", err)
			}
			for _, user := range users {
				if user.RoleID == nil {
					continue
				}
				membersByRole[*user.RoleID] = append(membersByRole[*user.RoleID], RoleMemberSummary{
					ID:        user.ID,
					FirstName: user.FirstName,
					LastName:  user.LastName,
					Email:     user.Email,
					JobTitle:  user.JobTitle,
					Department: func() string {
						if user.Department != nil {
							return user.Department.Name
						}
						return ""
					}(),
				})
			}
		}
	}

	for roleID := range membersByRole {
		sort.Slice(membersByRole[roleID], func(i, j int) bool {
			left := membersByRole[roleID][i]
			right := membersByRole[roleID][j]
			return left.FirstName+left.LastName+left.Email < right.FirstName+right.LastName+right.Email
		})
	}

	creatorIDs := make([]string, 0, len(roles))
	for _, role := range roles {
		if role.CreatedByUserID != nil {
			creatorIDs = append(creatorIDs, *role.CreatedByUserID)
		}
	}
	creatorIDs = uniqueStrings(creatorIDs)

	creators := make(map[string]models.User, len(creatorIDs))
	if len(creatorIDs) > 0 {
		var users []models.User
		if err := s.db.Where("id IN ?", creatorIDs).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("failed to load role creators: %w", err)
		}
		for _, user := range users {
			creators[user.ID] = user
		}
	}

	summaries := make([]RoleSummary, 0, len(roles))
	for _, role := range roles {
		members := membersByRole[role.ID]
		summary := RoleSummary{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			CreatedByUserID: func() string {
				if role.CreatedByUserID != nil {
					return *role.CreatedByUserID
				}
				return ""
			}(),
			CreatedAt:   role.CreatedAt,
			UpdatedAt:   role.UpdatedAt,
			MemberCount: int64(len(members)),
			Members:     members,
		}
		if role.CreatedByUserID != nil {
			if creator, ok := creators[*role.CreatedByUserID]; ok {
				summary.CreatedBy = actorSummaryFromUser(creator)
			} else {
				summary.CreatedBy = &ActorSummary{ID: *role.CreatedByUserID, Name: "User " + *role.CreatedByUserID}
			}
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (s *EmployeeService) UpdateDepartment(orgID, deptID, name, description string) (*models.Department, error) {
	var department models.Department
	if err := s.db.Where("organization_id = ? AND id = ?", orgID, deptID).First(&department).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: department %s in org %s", ErrNotFound, deptID, orgID)
		}
		return nil, fmt.Errorf("failed to load department: %w", err)
	}

	trimmedName := name
	if trimmedName == "" {
		trimmedName = department.Name
	}

	var existing models.Department
	err := s.db.Where("organization_id = ? AND name = ? AND id <> ?", orgID, trimmedName, deptID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: department %q already exists in this organization", ErrDuplicateDepartment, trimmedName)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to validate department uniqueness: %w", err)
	}

	department.UpdatedAt = time.Now()
	if err := s.db.Model(&models.Department{}).
		Where("organization_id = ? AND id = ?", orgID, deptID).
		Updates(map[string]interface{}{
			"name":        trimmedName,
			"description": description,
			"updated_at":  department.UpdatedAt,
		}).Error; err != nil {
		return nil, fmt.Errorf("failed to update department: %w", err)
	}
	department.Name = trimmedName
	department.Description = description
	return &department, nil
}

func (s *EmployeeService) DeleteDepartment(orgID, deptID string) error {
	var memberCount int64
	if err := s.db.Model(&models.User{}).Where("organization_id = ? AND department_id = ?", orgID, deptID).Count(&memberCount).Error; err != nil {
		return fmt.Errorf("failed to check department membership: %w", err)
	}
	if memberCount > 0 {
		return fmt.Errorf("department still has %d assigned employee(s)", memberCount)
	}

	result := s.db.Where("organization_id = ? AND id = ?", orgID, deptID).Delete(&models.Department{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete department: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: department %s in org %s", ErrNotFound, deptID, orgID)
	}
	return nil
}

func (s *EmployeeService) UpdateRole(orgID, roleID, name, description, updatedBy string, memberIDs []string) (*models.Role, error) {
	var role models.Role
	if err := s.db.Where("organization_id = ? AND id = ?", orgID, roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: role %s in org %s", ErrNotFound, roleID, orgID)
		}
		return nil, fmt.Errorf("failed to load role: %w", err)
	}

	trimmedName := name
	if trimmedName == "" {
		trimmedName = role.Name
	}

	var existing models.Role
	err := s.db.Where("organization_id = ? AND name = ? AND id <> ?", orgID, trimmedName, roleID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: role %q already exists in this organization", ErrDuplicateRole, trimmedName)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to validate role uniqueness: %w", err)
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		role.UpdatedAt = time.Now()
		if err := tx.Model(&models.Role{}).
			Where("organization_id = ? AND id = ?", orgID, roleID).
			Updates(map[string]interface{}{
				"name":        trimmedName,
				"description": description,
				"updated_at":  role.UpdatedAt,
			}).Error; err != nil {
			return fmt.Errorf("failed to update role: %w", err)
		}

		if hasUserRoleMembershipsTable(tx) {
			if err := tx.Where("organization_id = ? AND role_id = ?", orgID, roleID).Delete(&models.UserRoleMembership{}).Error; err != nil {
				return fmt.Errorf("failed to reset role memberships: %w", err)
			}
		} else {
			if err := tx.Model(&models.User{}).
				Where("organization_id = ? AND role_id = ?", orgID, roleID).
				Updates(map[string]interface{}{"role_id": nil, "updated_at": time.Now()}).Error; err != nil {
				return fmt.Errorf("failed to reset legacy role assignments: %w", err)
			}
		}

		if err := s.addUsersToRole(tx, orgID, roleID, updatedBy, memberIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	role.Name = trimmedName
	role.Description = description

	return &role, nil
}

func (s *EmployeeService) DeleteRole(orgID, roleID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if hasUserRoleMembershipsTable(tx) {
			if err := tx.Where("organization_id = ? AND role_id = ?", orgID, roleID).Delete(&models.UserRoleMembership{}).Error; err != nil {
				return fmt.Errorf("failed to remove role memberships: %w", err)
			}
		}
		if err := tx.Model(&models.User{}).Where("organization_id = ? AND role_id = ?", orgID, roleID).Update("role_id", nil).Error; err != nil {
			return fmt.Errorf("failed to clear legacy user role reference: %w", err)
		}
		result := tx.Where("organization_id = ? AND id = ?", orgID, roleID).Delete(&models.Role{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete role: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: role %s in org %s", ErrNotFound, roleID, orgID)
		}
		return nil
	})
}
