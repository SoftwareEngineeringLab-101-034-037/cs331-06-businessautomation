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
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	CreatedByUserID string        `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	MemberCount     int64         `json:"member_count"`
	CreatedBy       *ActorSummary `json:"created_by,omitempty"`
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
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	CreatedByUserID string              `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	MemberCount     int64               `json:"member_count"`
	CreatedBy       *ActorSummary       `json:"created_by,omitempty"`
	Members         []RoleMemberSummary `json:"members,omitempty"`
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

func normalizeName(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeNameKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeNameList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeName(value)
		if normalized == "" {
			continue
		}
		key := normalizeNameKey(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func (s *EmployeeService) CreateRole(orgID, name, description, createdBy string, memberIDs []string) (*models.Role, error) {
	trimmedName := normalizeName(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("role name is required")
	}
	normalizedNameKey := normalizeNameKey(trimmedName)

	var existing models.Role
	err := s.db.Where("organization_id = ? AND lower(trim(name)) = ?", orgID, normalizedNameKey).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: role %q already exists in this organization", ErrDuplicateRole, trimmedName)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing role: %w", err)
	}

	role := models.Role{
		OrganizationID: orgID,
		Name:           trimmedName,
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
				return fmt.Errorf("%w: role %q already exists in this organization", ErrDuplicateRole, trimmedName)
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
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).Create(&memberships).Error; err != nil {
		return fmt.Errorf("failed to assign users to role: %w", err)
	}

	return nil
}

func (s *EmployeeService) AssignRoleNamesToUser(tx *gorm.DB, orgID, userID, assignedBy string, roleNames []string) error {
	roleNames = normalizeNameList(roleNames)
	if len(roleNames) == 0 {
		return nil
	}

	normalizedRequested := make(map[string]string, len(roleNames))
	for _, roleName := range roleNames {
		normalizedRequested[normalizeNameKey(roleName)] = roleName
	}

	normalizedKeys := make([]string, 0, len(normalizedRequested))
	for key := range normalizedRequested {
		normalizedKeys = append(normalizedKeys, key)
	}

	var roles []models.Role
	if err := tx.Where("organization_id = ? AND lower(trim(name)) IN ?", orgID, normalizedKeys).Find(&roles).Error; err != nil {
		return fmt.Errorf("failed to resolve invited roles: %w", err)
	}

	resolved := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		resolved[normalizeNameKey(role.Name)] = struct{}{}
	}

	missing := make([]string, 0)
	for key, original := range normalizedRequested {
		if _, ok := resolved[key]; !ok {
			missing = append(missing, original)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("unknown role names: %v", missing)
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
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "user_id"}, {Name: "role_id"}},
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
	if len(departments) > 0 {
		deptIDs := make([]string, len(departments))
		for i, d := range departments {
			deptIDs[i] = d.ID
		}
		var countRows []struct {
			DepartmentID string `gorm:"column:department_id"`
			Count        int64  `gorm:"column:count"`
		}
		if err := s.db.Model(&models.User{}).
			Select("department_id, COUNT(*) AS count").
			Where("organization_id = ? AND department_id IN ?", orgID, deptIDs).
			Group("department_id").
			Scan(&countRows).Error; err != nil {
			return nil, fmt.Errorf("failed to count department members: %w", err)
		}
		for _, row := range countRows {
			memberCounts[row.DepartmentID] = row.Count
		}
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
		if err := s.db.Where("organization_id = ? AND id IN ?", orgID, creatorIDs).Find(&users).Error; err != nil {
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

	var memberships []models.UserRoleMembership
	if err := s.db.
		Where("organization_id = ?", orgID).
		Preload("User").
		Preload("User.Department").
		Find(&memberships).Error; err != nil {
		return nil, fmt.Errorf("failed to load role memberships: %w", err)
	}

	for _, membership := range memberships {
		if membership.User == nil {
			continue
		}
		membersByRole[membership.RoleID] = append(membersByRole[membership.RoleID], RoleMemberSummary{
			ID:        membership.User.ID,
			FirstName: membership.User.FirstName,
			LastName:  membership.User.LastName,
			Email:     membership.User.Email,
			JobTitle:  membership.User.JobTitle,
			Department: func() string {
				if membership.User.Department != nil {
					return membership.User.Department.Name
				}
				return ""
			}(),
		})
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
		if err := s.db.Where("organization_id = ? AND id IN ?", orgID, creatorIDs).Find(&users).Error; err != nil {
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

	trimmedName := normalizeName(name)
	if name == "" {
		trimmedName = department.Name
	} else if trimmedName == "" {
		return nil, fmt.Errorf("department name is required")
	}
	normalizedNameKey := normalizeNameKey(trimmedName)

	var existing models.Department
	err := s.db.Where("organization_id = ? AND lower(trim(name)) = ? AND id <> ?", orgID, normalizedNameKey, deptID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: department %q already exists in this organization", ErrDuplicateDepartment, trimmedName)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
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
	result := s.db.Where(
		"organization_id = ? AND id = ? AND NOT EXISTS (?)",
		orgID,
		deptID,
		s.db.Model(&models.User{}).
			Select("1").
			Where("organization_id = ? AND department_id = ?", orgID, deptID),
	).Delete(&models.Department{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete department: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var memberCount int64
		if err := s.db.Model(&models.User{}).Where("organization_id = ? AND department_id = ?", orgID, deptID).Count(&memberCount).Error; err != nil {
			return fmt.Errorf("failed to check department membership: %w", err)
		}
		if memberCount > 0 {
			return fmt.Errorf("department still has %d assigned employee(s)", memberCount)
		}
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

	trimmedName := normalizeName(name)
	if name == "" {
		trimmedName = role.Name
	} else if trimmedName == "" {
		return nil, fmt.Errorf("role name is required")
	}
	normalizedNameKey := normalizeNameKey(trimmedName)

	var existing models.Role
	err := s.db.Where("organization_id = ? AND lower(trim(name)) = ? AND id <> ?", orgID, normalizedNameKey, roleID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: role %q already exists in this organization", ErrDuplicateRole, trimmedName)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
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

		// Diff-based update: read existing, add new, remove stale.
		var existingUserIDs []string
		if err := tx.Model(&models.UserRoleMembership{}).
			Where("organization_id = ? AND role_id = ?", orgID, roleID).
			Pluck("user_id", &existingUserIDs).Error; err != nil {
			return fmt.Errorf("failed to read existing role memberships: %w", err)
		}

		existingSet := make(map[string]struct{}, len(existingUserIDs))
		for _, uid := range existingUserIDs {
			existingSet[uid] = struct{}{}
		}
		intendedSet := make(map[string]struct{}, len(memberIDs))
		for _, uid := range uniqueStrings(memberIDs) {
			intendedSet[uid] = struct{}{}
		}

		var toRemove []string
		for uid := range existingSet {
			if _, ok := intendedSet[uid]; !ok {
				toRemove = append(toRemove, uid)
			}
		}
		var toAdd []string
		for uid := range intendedSet {
			if _, ok := existingSet[uid]; !ok {
				toAdd = append(toAdd, uid)
			}
		}

		if len(toRemove) > 0 {
			if err := tx.Where("organization_id = ? AND role_id = ? AND user_id IN ?", orgID, roleID, toRemove).
				Delete(&models.UserRoleMembership{}).Error; err != nil {
				return fmt.Errorf("failed to remove stale role memberships: %w", err)
			}
		}
		if len(toAdd) > 0 {
			if err := s.addUsersToRole(tx, orgID, roleID, updatedBy, toAdd); err != nil {
				return err
			}
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
		if err := tx.Where("organization_id = ? AND role_id = ?", orgID, roleID).Delete(&models.UserRoleMembership{}).Error; err != nil {
			return fmt.Errorf("failed to remove role memberships: %w", err)
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
