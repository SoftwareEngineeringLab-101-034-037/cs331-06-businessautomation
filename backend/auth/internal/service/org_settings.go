package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrganizationSettingsUpdateInput struct {
	Domain   string
	Industry string
	Size     string
	Country  string
	UseCase  string
}

func trimOrgSettingValue(value string) string {
	return strings.TrimSpace(value)
}

func (s *EmployeeService) GetOrganizationSettings(orgID string) (*models.OrganizationSettings, error) {
	normalizedOrgID := strings.TrimSpace(orgID)
	if normalizedOrgID == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	now := time.Now()
	seed := models.OrganizationSettings{
		ID:             uuid.NewString(),
		OrganizationID: normalizedOrgID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}},
		DoNothing: true,
	}).Create(&seed).Error; err != nil {
		return nil, fmt.Errorf("failed to ensure organization settings row: %w", err)
	}

	var settings models.OrganizationSettings
	err := s.db.Where("organization_id = ?", normalizedOrgID).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to load organization settings after upsert")
		}
		return nil, fmt.Errorf("failed to get organization settings: %w", err)
	}

	return &settings, nil
}

func (s *EmployeeService) UpdateOrganizationSettings(orgID string, input OrganizationSettingsUpdateInput) (*models.OrganizationSettings, error) {
	normalizedOrgID := strings.TrimSpace(orgID)
	settings, err := s.GetOrganizationSettings(normalizedOrgID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"domain":     trimOrgSettingValue(input.Domain),
		"industry":   trimOrgSettingValue(input.Industry),
		"size":       trimOrgSettingValue(input.Size),
		"country":    trimOrgSettingValue(input.Country),
		"use_case":   trimOrgSettingValue(input.UseCase),
		"updated_at": time.Now(),
	}

	if err := s.db.Model(&models.OrganizationSettings{}).
		Where("organization_id = ?", normalizedOrgID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update organization settings: %w", err)
	}

	if err := s.db.Where("organization_id = ?", normalizedOrgID).First(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to reload organization settings: %w", err)
	}

	return settings, nil
}
