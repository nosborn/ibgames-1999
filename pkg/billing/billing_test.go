package billing

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/nosborn/ibgames-1999/pkg/db"
	"github.com/nosborn/ibgames-1999/pkg/ibgames"
)

func setupTestDB(t *testing.T) (string, *sql.DB) {
	tempFile, err := os.CreateTemp("", "test_billing_*.db")
	require.NoError(t, err)
	tempFile.Close()

	// Connect via db package for code under test
	err = db.Connect(tempFile.Name(), false)
	require.NoError(t, err)

	// Create direct connection for test verification
	testDB, err := sql.Open("sqlite", tempFile.Name())
	require.NoError(t, err)

	// Create test tables using direct connection
	_, err = testDB.Exec(`
		CREATE TABLE accounts (
			uid INTEGER PRIMARY KEY,
			data TEXT
		)
	`)
	require.NoError(t, err)

	_, err = testDB.Exec(`
		CREATE TABLE sessions (
			sid INTEGER PRIMARY KEY AUTOINCREMENT,
			data TEXT,
			end TIMESTAMP,
			minutes INTEGER
		)
	`)
	require.NoError(t, err)

	return tempFile.Name(), testDB
}

func teardownTestDB(t *testing.T, testDB *sql.DB, dbFile string) {
	testDB.Close()
	db.Exit()
	os.Remove(dbFile)

	// Reset prepared statement state for next test
	prepared = false
	insertStmt = nil
	selectStmt = nil
	update1Stmt = nil
	update2Stmt = nil
}

func createTestAccount(t *testing.T, testDB *sql.DB, uid ibgames.AccountID, complimentary string, minutes int) {
	_, err := testDB.Exec(`
		INSERT INTO accounts (uid, data)
		VALUES (?, json_object('complimentary', ?, 'minutes', ?))
	`, uid, complimentary, minutes)
	require.NoError(t, err)
}

func getAccountMinutes(t *testing.T, testDB *sql.DB, uid ibgames.AccountID) int {
	var minutes int
	err := testDB.QueryRow("SELECT json_extract(data, '$.minutes') FROM accounts WHERE uid = ?", uid).Scan(&minutes)
	require.NoError(t, err)
	return minutes
}

func TestInit(t *testing.T) {
	dbFile, testDB := setupTestDB(t)
	defer teardownTestDB(t, testDB, dbFile)

	t.Run("init with Federation product", func(t *testing.T) {
		// Reset prepared state
		prepared = false

		err := Init(Federation)
		require.NoError(t, err)
		assert.True(t, prepared)

		// Should be idempotent
		err = Init(Federation)
		assert.NoError(t, err)
	})

	t.Run("init with AgeOfAdventure product", func(t *testing.T) {
		// Reset prepared state
		prepared = false

		err := Init(AgeOfAdventure)
		require.NoError(t, err)
		assert.True(t, prepared)
	})
}

func TestAutoCommitAndFreePeriod(t *testing.T) {
	t.Run("AutoCommit sets flag", func(t *testing.T) {
		AutoCommit(true)
		assert.True(t, autoCommit)

		AutoCommit(false)
		assert.False(t, autoCommit)
	})

	t.Run("FreePeriod sets flag", func(t *testing.T) {
		FreePeriod(true)
		assert.True(t, freePeriod)

		FreePeriod(false)
		assert.False(t, freePeriod)
	})
}

func TestBeginSession(t *testing.T) {
	dbFile, testDB := setupTestDB(t)
	defer teardownTestDB(t, testDB, dbFile)

	err := Init(Federation)
	require.NoError(t, err)

	t.Run("begin session for complimentary account", func(t *testing.T) {
		uid := ibgames.AccountID(666000)
		createTestAccount(t, testDB, uid, "Y", 1000)

		session, err := BeginSession(uid, "192.0.2.1")
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, uid, session.uid)
		assert.True(t, session.complimentary)
		assert.Equal(t, 0, session.lastCharge)
		assert.True(t, session.ticking)
		assert.Positive(t, session.sid)
	})

	t.Run("begin session for paying account", func(t *testing.T) {
		// Reset free period
		FreePeriod(false)

		uid := ibgames.AccountID(666001)
		createTestAccount(t, testDB, uid, "N", 1000)

		session, err := BeginSession(uid, "192.0.2.2")
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, uid, session.uid)
		assert.False(t, session.complimentary)
		assert.Equal(t, minimumCharge, session.lastCharge)
		assert.True(t, session.ticking)

		// Check that minutes were deducted from account using direct DB connection
		minutes := getAccountMinutes(t, testDB, uid)
		assert.Equal(t, 1000-minimumCharge, minutes)
	})

	t.Run("begin session during free period", func(t *testing.T) {
		FreePeriod(true)
		defer FreePeriod(false)

		uid := ibgames.AccountID(666002)
		createTestAccount(t, testDB, uid, "N", 1000)

		session, err := BeginSession(uid, "192.0.2.3")
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, uid, session.uid)
		assert.False(t, session.complimentary)
		assert.Equal(t, 0, session.lastCharge)

		// Check that minutes were NOT deducted during free period
		minutes := getAccountMinutes(t, testDB, uid)
		assert.Equal(t, 1000, minutes)
	})

	t.Run("begin session for non-existent account", func(t *testing.T) {
		uid := ibgames.AccountID(999999)

		session, err := BeginSession(uid, "192.0.2.4")
		require.Error(t, err)
		assert.Nil(t, session)
	})
}

func TestSessionMethods(t *testing.T) {
	dbFile, testDB := setupTestDB(t)
	defer teardownTestDB(t, testDB, dbFile)

	err := Init(Federation)
	require.NoError(t, err)

	// Use a paying account (complimentary = "N") for time accumulation testing
	FreePeriod(false)
	uid := ibgames.AccountID(666000)
	createTestAccount(t, testDB, uid, "N", 1000)

	session, err := BeginSession(uid, "192.168.1.1")
	require.NoError(t, err)

	t.Run("StartClock and StopClock", func(t *testing.T) {
		// Session should start with clock running
		assert.True(t, session.ticking)

		session.StopClock()
		assert.False(t, session.ticking)

		session.StartClock()
		assert.True(t, session.ticking)

		// StartClock should be idempotent
		session.StartClock()
		assert.True(t, session.ticking)

		// StopClock should be idempotent
		session.StopClock()
		session.StopClock()
		assert.False(t, session.ticking)
	})

	t.Run("Tick accumulates time", func(t *testing.T) {
		// Start fresh
		session.seconds = 0
		session.lastTick = time.Now().Unix() - 120 // Simulate 2 minutes ago
		session.ticking = true

		result := session.Tick()
		assert.Equal(t, 1, result)

		// Should have accumulated at least 120 seconds
		assert.GreaterOrEqual(t, session.seconds, int64(120))
	})

	t.Run("Time returns charged time", func(t *testing.T) {
		chargedTime := session.Time()
		assert.Equal(t, session.lastCharge, chargedTime)
	})

	t.Run("End returns final time", func(t *testing.T) {
		endTime := session.End()
		assert.Equal(t, session.lastCharge, endTime)
	})
}

// Basic unit tests - always run, test core functionality quickly
func TestSessionBillingLogic(t *testing.T) {
	dbFile, testDB := setupTestDB(t)
	defer teardownTestDB(t, testDB, dbFile)

	err := Init(Federation)
	require.NoError(t, err)

	t.Run("complimentary account accumulates no time", func(t *testing.T) {
		uid := ibgames.AccountID(666100)
		createTestAccount(t, testDB, uid, "Y", 1000)

		session, err := BeginSession(uid, "192.0.2.1")
		require.NoError(t, err)

		// Complimentary account should not have deducted minutes at start
		minutes := getAccountMinutes(t, testDB, uid)
		assert.Equal(t, 1000, minutes)

		// Simulate time passage - should not accumulate for complimentary
		session.lastTick = time.Now().Unix() - 120
		session.ticking = true

		result := session.Tick()
		assert.Equal(t, 1, result)
		assert.Equal(t, int64(0), session.seconds)
	})

	t.Run("paying account accumulates time when ticking", func(t *testing.T) {
		FreePeriod(false)

		uid := ibgames.AccountID(666101)
		createTestAccount(t, testDB, uid, "N", 1000)

		session, err := BeginSession(uid, "192.0.2.1")
		require.NoError(t, err)

		// Should have deducted minimum charge at start
		minutes := getAccountMinutes(t, testDB, uid)
		assert.Equal(t, 1000-minimumCharge, minutes)

		// Simulate time passage when ticking=true
		session.lastTick = time.Now().Unix() - 120
		session.ticking = true

		result := session.Tick()
		assert.Equal(t, 1, result)
		assert.GreaterOrEqual(t, session.seconds, int64(120))
	})

	t.Run("paying account does not accumulate when ticking=false", func(t *testing.T) {
		FreePeriod(false)

		uid := ibgames.AccountID(666103)
		createTestAccount(t, testDB, uid, "N", 1000)

		session, err := BeginSession(uid, "192.0.2.1")
		require.NoError(t, err)

		// Stop the clock
		session.StopClock()
		initial := session.seconds

		// Simulate time passage when ticking=false
		session.lastTick = time.Now().Unix() - 120

		result := session.Tick()
		assert.Equal(t, 1, result)
		// Should not have accumulated time since ticking=false
		assert.Equal(t, initial, session.seconds)
	})

	t.Run("free period prevents time accumulation", func(t *testing.T) {
		FreePeriod(true)
		defer FreePeriod(false)

		uid := ibgames.AccountID(666102)
		createTestAccount(t, testDB, uid, "N", 1000)

		session, err := BeginSession(uid, "192.0.2.1")
		require.NoError(t, err)

		// Should not have deducted minutes during free period
		minutes := getAccountMinutes(t, testDB, uid)
		assert.Equal(t, 1000, minutes)

		// Simulate time passage - should not accumulate during free period
		session.lastTick = time.Now().Unix() - 120
		session.ticking = true

		result := session.Tick()
		assert.Equal(t, 1, result)
		assert.Equal(t, int64(0), session.seconds)
	})
}
