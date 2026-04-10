package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
	"gorm.io/gorm"
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
	if strings.TrimSpace(orgID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}

	var settings models.OrganizationSettings
	err := s.db.Where("organization_id = ?", orgID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		settings = models.OrganizationSettings{
			OrganizationID: orgID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if createErr := s.db.Create(&settings).Error; createErr != nil {
			return nil, fmt.Errorf("failed to create organization settings: %w", createErr)
		}
		return &settings, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization settings: %w", err)
	}

	return &settings, nil
}

func (s *EmployeeService) UpdateOrganizationSettings(orgID string, input OrganizationSettingsUpdateInput) (*models.OrganizationSettings, error) {
	settings, err := s.GetOrganizationSettings(orgID)
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
		Where("organization_id = ?", orgID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update organization settings: %w", err)
	}

	if err := s.db.Where("organization_id = ?", orgID).First(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to reload organization settings: %w", err)
	}

	return settings, nil
}
