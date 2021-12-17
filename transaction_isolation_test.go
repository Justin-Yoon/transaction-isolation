package transaction_isolation

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
)

var (
	pool *pgxpool.Pool
	ctx  context.Context
)

func TestMain(m *testing.M) {
	ctx = context.Background()
	conf, err := pgxpool.ParseConfig("postgresql://postgres:password@localhost:5433/postgres")
	if err != nil {
		panic(err)
	}
	_pool, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		panic(err)
	}
	pool = _pool
	defer pool.Close()

	_, err = pool.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS dev;
DROP TABLE IF EXISTS dev.balances;
CREATE TABLE dev.balances (
	name TEXT NOT NULL PRIMARY KEY,
	value int NOT NULL
);
	`)

	if err != nil {
		panic(err)
	}

	m.Run()
}

func resetTable() {
	_, err := pool.Exec(ctx, `
TRUNCATE dev.balances RESTART IDENTITY CASCADE;
INSERT INTO dev.balances VALUES ('Alice', 100);
INSERT INTO dev.balances VALUES ('Bob', 100);
	`)
	if err != nil {
		panic(err)
	}
}

func beginTransactions(isoLevel pgx.TxIsoLevel) (pgx.Tx, pgx.Tx) {
	tx1, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	if err != nil {
		panic(err)
	}
	tx2, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
	if err != nil {
		panic(err)
	}

	return tx1, tx2
}

/*
	ReadCommited only guarantees that all reads are commited (not dirty), any subsequent reads may see new commmited changes.
	This is the default isolation level for Postgres
*/
func TestReadCommited(t *testing.T) {
	t.Run("no dirty read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.ReadCommitted)

		anomaly := DirtyRead(ctx, tx1, tx2)

		assert.False(t, anomaly)
	})

	t.Run("non repeatable read possible", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.ReadCommitted)

		anomaly := NonRepeatableRead(ctx, tx1, tx2)

		assert.True(t, anomaly)
	})

	t.Run("phantom read possible", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.ReadCommitted)

		anomaly := PhantomRead(ctx, tx1, tx2)

		assert.True(t, anomaly)
	})

	t.Run("lost update possible", func(t *testing.T) {
		tx1, tx2 := beginTransactions(pgx.ReadCommitted)

		anomaly, err := LostUpdate(ctx, pool, tx1, tx2)

		assert.NoError(t, err)
		assert.True(t, anomaly)
	})

	t.Run("write skew possible", func(t *testing.T) {
		tx1, tx2 := beginTransactions(pgx.ReadCommitted)

		anomaly, err := WriteSkew(ctx, pool, tx1, tx2)

		assert.NoError(t, err)
		assert.True(t, anomaly)
	})

}

/*
	RepeatableRead works via snapshot isolation.
	You can think of this as each transaction, when it starts, stores a snapshot of the database at that point in time.
	This is achieved via a technique called multi version concurrency control (MVCC)
*/
func TestRepeatableRead(t *testing.T) {
	t.Run("no dirty read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.RepeatableRead)

		anomaly := DirtyRead(ctx, tx1, tx2)

		assert.False(t, anomaly)
	})

	t.Run("no non repeatable read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.RepeatableRead)

		anomaly := NonRepeatableRead(ctx, tx1, tx2)

		assert.False(t, anomaly)
	})

	t.Run("no phantom read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.RepeatableRead)

		phantomRead := PhantomRead(ctx, tx1, tx2)

		assert.False(t, phantomRead)
	})

	t.Run("lost update caught and error returned", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.RepeatableRead)

		_, err := LostUpdate(ctx, pool, tx1, tx2)
		assert.Error(t, err)

		var pgErr *pgconn.PgError
		errors.As(err, &pgErr)

		// concurrent update error
		assert.Equal(t, "40001", pgErr.Code)
		assert.Equal(t, "could not serialize access due to concurrent update", pgErr.Message)
	})

	t.Run("write skew possible", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.RepeatableRead)
		writeSkew, err := WriteSkew(ctx, pool, tx1, tx2)

		assert.NoError(t, err)
		assert.True(t, writeSkew)
	})
}

/*
	The Serializable level builds upon the RepeatableRead level by implementing predicate locks
	into the prevously mentioned Snapshot Isolation. This is called Serializable Snapshot Isolation.
*/
func TestSerializable(t *testing.T) {
	t.Run("no dirty read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.Serializable)

		anomaly := DirtyRead(ctx, tx1, tx2)

		assert.False(t, anomaly)
	})

	t.Run("no non repeatable read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.Serializable)

		anomaly := NonRepeatableRead(ctx, tx1, tx2)

		assert.False(t, anomaly)
	})

	t.Run("no phantom read", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.Serializable)

		anomaly := PhantomRead(ctx, tx1, tx2)

		assert.False(t, anomaly)
	})

	t.Run("lost update caught and error returned", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.Serializable)

		_, err := LostUpdate(ctx, pool, tx1, tx2)
		assert.Error(t, err)

		var pgErr *pgconn.PgError
		errors.As(err, &pgErr)

		// concurrent update error
		assert.Equal(t, "40001", pgErr.Code)
		assert.Equal(t, "could not serialize access due to concurrent update", pgErr.Message)
	})

	t.Run("write skew caught and error returned", func(t *testing.T) {
		resetTable()
		tx1, tx2 := beginTransactions(pgx.Serializable)

		_, err := WriteSkew(ctx, pool, tx1, tx2)
		assert.Error(t, err)

		var pgErr *pgconn.PgError
		errors.As(err, &pgErr)

		// concurrent update error
		assert.Equal(t, "40001", pgErr.Code)
		assert.Equal(t, "could not serialize access due to read/write dependencies among transactions", pgErr.Message)
	})
}
