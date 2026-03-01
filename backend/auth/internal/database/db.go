package database

import (
	"errors"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

var DB *gorm.DB

var openDatabase = func(databaseURL string) (*gorm.DB, error) {
	// Supabase uses PgBouncer (port 6543) in transaction mode which doesn't
	// support prepared statements. We disable them to avoid "prepared statement
	// already exists" errors.
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  databaseURL,
		PreferSimpleProtocol: true, // disables prepared statement caching
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
}

var runAutoMigrate = func(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.OrganizationSettings{},
		&models.Department{},
		&models.EmployeeInvitation{},
	)
}

// Connect establishes connection to Supabase PostgreSQL
func Connect(databaseURL string) error {
	var err error

	DB, err = openDatabase(databaseURL)
	if err != nil {
		return err
	}

	log.Println("Connected to Supabase PostgreSQL")
	return nil
}

// Migrate runs database migrations
func Migrate() error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}

	err := runAutoMigrate(DB)
	if err != nil {
		return err
	}

	log.Println("Database migrations completed")
	return nil
}
