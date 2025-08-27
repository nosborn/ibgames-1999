// Package testutil provides common testing utilities for the project.
package testutil

import (
	"database/sql"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/nosborn/ibgames-1999"
)

// DatabaseSetup holds references to test database resources.
type DatabaseSetup struct {
	FilePath string
	TestDB   *sql.DB
}

// SetupTestDatabase creates a temporary SQLite database for testing.
// It returns a direct SQL connection for test verification.
// The caller is responsible for setting up any package-specific connections.
func SetupTestDatabase(t *testing.T) *DatabaseSetup {
	tempFile, err := os.CreateTemp("", "test_accounts_*.db")
	require.NoError(t, err)
	tempFile.Close()

	// Create direct connection for test verification
	testDB, err := sql.Open("sqlite", tempFile.Name())
	require.NoError(t, err)

	setup := &DatabaseSetup{
		FilePath: tempFile.Name(),
		TestDB:   testDB,
	}

	t.Cleanup(func() {
		setup.Teardown()
	})

	return setup
}

// SetupTestDatabaseWithSchema creates a test database and initializes it with the accounts schema.
func SetupTestDatabaseWithSchema(t *testing.T) *DatabaseSetup {
	setup := SetupTestDatabase(t)
	setup.CreateSchema(t)
	return setup
}

// CreateSchema creates the accounts and sessions tables using the production schema file.
func (d *DatabaseSetup) CreateSchema(t *testing.T) {
	// Close the Go connection temporarily to avoid conflicts
	d.TestDB.Close()

	// Find the schema file - try different possible locations
	schemaPath := "accounts.sql"
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		schemaPath = "../accounts.sql"
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			schemaPath = "../../accounts.sql"
			if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
				require.NoError(t, err, "Could not find accounts.sql schema file")
			}
		}
	}

	// Use sqlite3 CLI to load the production schema
	cmd := exec.Command("sqlite3", d.FilePath, ".read "+schemaPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("sqlite3 output: %s", string(output))
		require.NoError(t, err, "Failed to load schema using sqlite3")
	}

	// Reopen the Go connection
	d.TestDB, err = sql.Open("sqlite", d.FilePath)
	require.NoError(t, err)
}

// CreateTestAccount inserts a test account into the database.
func (d *DatabaseSetup) CreateTestAccount(t *testing.T, uid ibgames.AccountID, name, complimentary string, minutes int) {
	_, err := d.TestDB.Exec(`
		INSERT INTO accounts (uid, name, encrypt, complimentary, minutes)
		VALUES (?, ?, 'dummy_hash', ?, ?)
	`, uid, name, complimentary, minutes)
	require.NoError(t, err)
}

// Teardown cleans up database resources.
func (d *DatabaseSetup) Teardown() {
	if d.TestDB != nil {
		d.TestDB.Close()
	}
	if d.FilePath != "" {
		os.Remove(d.FilePath)
	}
}
