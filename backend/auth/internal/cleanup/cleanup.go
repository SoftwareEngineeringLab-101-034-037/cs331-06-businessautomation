package cleanup

import (
	"log"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

// Config holds cleanup configuration
type Config struct {
	Interval      time.Duration // How often to run cleanup
	RetentionDays int           // Delete soft-deleted records older than this
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Interval:      5 * time.Minute,
		RetentionDays: 30, // Keep soft-deleted records for 30 days
	}
}

// Start begins the cleanup background job
// This runs in a goroutine and cleans up soft-deleted records
func Start(cfg Config) {
	go func() {
		log.Printf("Cleanup job started (interval: %v, retention: %d days)", cfg.Interval, cfg.RetentionDays)

		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		// Run immediately on start, then on interval
		runCleanup(cfg.RetentionDays)

		for range ticker.C {
			runCleanup(cfg.RetentionDays)
		}
	}()
}

// runCleanup performs the actual cleanup of old soft-deleted records
func runCleanup(retentionDays int) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	// Clean up soft-deleted users
	result := database.DB.Unscoped().Where("is_active = ? AND updated_at < ?", false, cutoffDate).Delete(&models.User{})
	if result.Error != nil {
		log.Printf("Cleanup error (users): %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Cleanup: deleted %d inactive users older than %d days", result.RowsAffected, retentionDays)
	}

	// Clean up soft-deleted organizations
	result = database.DB.Unscoped().Where("is_active = ? AND updated_at < ?", false, cutoffDate).Delete(&models.Organization{})
	if result.Error != nil {
		log.Printf("Cleanup error (organizations): %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Cleanup: deleted %d inactive organizations older than %d days", result.RowsAffected, retentionDays)
	}

	// Expire pending invitations that are past their expiry time
	result = database.DB.Model(&models.EmployeeInvitation{}).
		Where("status = ? AND expires_at < ?", "pending", time.Now()).
		Update("status", "expired")
	if result.Error != nil {
		log.Printf("Cleanup error (invitations): %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Cleanup: expired %d old invitations", result.RowsAffected)
	}
}

