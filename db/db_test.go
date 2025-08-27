package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nosborn/ibgames-1999/internal/testutil"
)

func TestConnect(t *testing.T) {
	t.Run("connect to valid database path", func(t *testing.T) {
		setup := testutil.SetupTestDatabase(t)

		err := Connect(setup.FilePath, false)
		require.NoError(t, err)

		// Should have a transaction ready
		assert.NotNil(t, Tx())

		// Verify we can query the database
		_, err = Exec("SELECT 1")
		assert.NoError(t, err)
	})

	t.Run("connect to invalid directory path fails", func(t *testing.T) {
		err := Connect("/nonexistent/path/db.sqlite", false)
		assert.Error(t, err)
	})
}

func TestTransactionOperations(t *testing.T) {
	setup := testutil.SetupTestDatabase(t)
	require.NoError(t, Connect(setup.FilePath, false))

	t.Run("commit restarts transaction", func(t *testing.T) {
		oldTx := Tx()

		err := Commit()
		require.NoError(t, err)

		newTx := Tx()
		assert.NotNil(t, newTx)
		assert.NotEqual(t, oldTx, newTx) // Should be a new transaction
	})

	t.Run("rollback restarts transaction", func(t *testing.T) {
		oldTx := Tx()

		err := Rollback()
		require.NoError(t, err)

		newTx := Tx()
		assert.NotNil(t, newTx)
		assert.NotEqual(t, oldTx, newTx) // Should be a new transaction
	})
}

func TestExec(t *testing.T) {
	setup := testutil.SetupTestDatabase(t)
	require.NoError(t, Connect(setup.FilePath, false))

	t.Run("exec creates and inserts into table", func(t *testing.T) {
		// Create a simple test table
		_, err := Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
		require.NoError(t, err)

		// Insert data
		result, err := Exec("INSERT INTO test (name) VALUES (?)", "test_name")
		require.NoError(t, err)

		id, err := result.LastInsertId()
		require.NoError(t, err)
		assert.Equal(t, int64(1), id)

		rows, err := result.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), rows)
	})
}

func TestDisconnectAndExit(t *testing.T) {
	setup := testutil.SetupTestDatabase(t)

	t.Run("disconnect commits transaction", func(t *testing.T) {
		err := Disconnect()
		assert.NoError(t, err)
	})

	t.Run("exit cleans up resources", func(t *testing.T) {
		// Reconnect for this test
		require.NoError(t, Connect(setup.FilePath, false))

		err := Exit()
		require.NoError(t, err)

		// After exit, tx should be nil
		assert.Nil(t, Tx())
	})
}
