package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestConnectReturnsErrorForInvalidURL(t *testing.T) {
	restoreGlobals(t)

	err := Connect("://invalid-dsn")
	if err == nil {
		t.Fatal("expected Connect to return an error for invalid DSN")
	}
}

func TestConnectSetsDBOnSuccess(t *testing.T) {
	restoreGlobals(t)

	testDB := newSQLiteDB(t)
	openDatabase = func(_ string) (*gorm.DB, error) {
		return testDB, nil
	}

	if err := Connect("any-connection-string"); err != nil {
		t.Fatalf("expected Connect to succeed, got: %v", err)
	}
	if DB != testDB {
		t.Fatal("expected global DB to be set to opened database")
	}
}

func TestConnectPropagatesOpenError(t *testing.T) {
	restoreGlobals(t)

	openErr := errors.New("open failure")
	openDatabase = func(_ string) (*gorm.DB, error) {
		return nil, openErr
	}

	err := Connect("any-connection-string")
	if err == nil {
		t.Fatal("expected Connect error")
	}
	if !errors.Is(err, openErr) {
		t.Fatalf("expected open error, got: %v", err)
	}
	if DB != nil {
		t.Fatal("expected DB to remain nil on failed connect")
	}
}

func TestMigrateReturnsErrorWhenDBNotInitialized(t *testing.T) {
	restoreGlobals(t)
	DB = nil

	err := Migrate()
	if err == nil {
		t.Fatal("expected error when DB is not initialized")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("expected initialization error, got: %v", err)
	}
}

func TestMigratePropagatesAutoMigrateError(t *testing.T) {
	restoreGlobals(t)
	DB = newSQLiteDB(t)

	migrateErr := errors.New("migration failed")
	runAutoMigrate = func(_ *gorm.DB) error {
		return migrateErr
	}

	err := Migrate()
	if err == nil {
		t.Fatal("expected migration error")
	}
	if !errors.Is(err, migrateErr) {
		t.Fatalf("expected migrate error, got: %v", err)
	}
}

func TestMigrateCallsAutoMigrateAndSucceeds(t *testing.T) {
	restoreGlobals(t)
	DB = newSQLiteDB(t)

	called := false
	runAutoMigrate = func(db *gorm.DB) error {
		called = true
		if db != DB {
			t.Fatal("expected Migrate to pass global DB to runAutoMigrate")
		}
		return nil
	}

	if err := Migrate(); err != nil {
		t.Fatalf("expected Migrate to succeed, got: %v", err)
	}
	if !called {
		t.Fatal("expected runAutoMigrate to be called")
	}
}

func TestRunAutoMigrateCreatesRoleAndMembershipTables(t *testing.T) {
	testDB := newSQLiteDB(t)

	if err := runAutoMigrate(testDB); err != nil {
		// SQLite does not support some PostgreSQL-oriented default expressions in model tags.
		if strings.Contains(strings.ToLower(err.Error()), "syntax error") {
			t.Skipf("skipping on sqlite due to postgres-specific migration SQL: %v", err)
		}
		t.Fatalf("expected runAutoMigrate to succeed, got: %v", err)
	}

	if !testDB.Migrator().HasTable(&models.Role{}) {
		t.Fatal("expected roles table to be migrated")
	}

	if !testDB.Migrator().HasTable(&models.UserRoleMembership{}) {
		t.Fatal("expected user_role_memberships table to be migrated")
	}
}

func newSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()

	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return testDB
}

func restoreGlobals(t *testing.T) {
	t.Helper()

	previousDB := DB
	previousOpenDatabase := openDatabase
	previousRunAutoMigrate := runAutoMigrate

	t.Cleanup(func() {
		DB = previousDB
		openDatabase = previousOpenDatabase
		runAutoMigrate = previousRunAutoMigrate
	})
}
