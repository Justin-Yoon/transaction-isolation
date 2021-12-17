package transaction_isolation

import (
	"context"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	Alice = "Alice"
	Bob   = "Bob"
)

type Balance struct {
	Name  string `db:"name"`
	Value int    `db:"value"`
}

/*
	Where a transaction sees another transactions uncommited change (This should not be possible at any isolation level in postgres)
*/
func DirtyRead(ctx context.Context, tx1, tx2 pgx.Tx) bool {
	defer tx1.Rollback(ctx)
	defer tx2.Rollback(ctx)
	// set Alice balance but don't commit it
	err := SetBalance(ctx, tx1, Alice, 150)
	if err != nil {
		panic(err)
	}

	// fetch Alice balance from different transaction
	aliceBal := GetBalance(ctx, tx2, Alice)

	// it is dirty if aliceBal reflects the uncommited value
	return aliceBal.Value == 150
}

/*
	Aka read skew. Where a transaction gets a value twice and get's different results
*/
func NonRepeatableRead(ctx context.Context, tx1, tx2 pgx.Tx) bool {
	defer tx1.Rollback(ctx)
	defer tx2.Rollback(ctx)
	// tx1 gets first value
	aliceBal1 := GetBalance(ctx, tx1, Alice)

	// tx2 commits changed value in the middle of tx1
	err := SetBalance(ctx, tx2, Alice, 150)
	if err != nil {
		panic(err)
	}
	err = tx2.Commit(ctx)
	if err != nil {
		panic(err)
	}

	// tx1 gets second value
	aliceBal2 := GetBalance(ctx, tx1, Alice)
	err = tx1.Commit(ctx)
	if err != nil {
		panic(err)
	}
	return aliceBal1.Value != aliceBal2.Value
}

/*
	Same as for read skew, but for a range of values
*/
func PhantomRead(ctx context.Context, tx1, tx2 pgx.Tx) bool {
	defer tx1.Rollback(ctx)
	defer tx2.Rollback(ctx)
	// tx1 gets first set of values
	negatives1 := GetNegativeBalances(ctx, tx1)

	// tx2 commits changed value in the middle of tx1
	err := SetBalance(ctx, tx2, Alice, -100)
	if err != nil {
		panic(err)
	}
	err = tx2.Commit(ctx)
	if err != nil {
		panic(err)
	}

	// tx1 gets second set of values
	negatives2 := GetNegativeBalances(ctx, tx1)
	err = tx1.Commit(ctx)
	if err != nil {
		panic(err)
	}

	return len(negatives1) == 0 && len(negatives2) == 1
}

/*
	Two transactions update the same row, the first update is lost
*/
func LostUpdate(ctx context.Context, pool *pgxpool.Pool, tx1, tx2 pgx.Tx) (bool, error) {
	defer tx1.Rollback(ctx)
	defer tx2.Rollback(ctx)

	// Tx1 reads Alice balance
	aliceBal := GetBalance(ctx, tx1, Alice)
	// Tx2 reads Alice balance
	aliceBal2 := GetBalance(ctx, tx2, Alice)

	// Tx1 writes to Alice Balance
	err := SetBalance(ctx, tx1, Alice, aliceBal.Value+50)
	if err != nil {
		panic(err)
	}
	err = tx1.Commit(ctx)
	if err != nil {
		panic(err)
	}

	// Tx2 tries to write to Alice Balance thereby overwriting Tx1's write
	err = SetBalance(ctx, tx2, Alice, aliceBal2.Value+100)
	if err != nil {
		// This is expected for RepeatableRead Isolation and above
		return false, err
	}
	err = tx2.Commit(ctx)
	if err != nil {
		panic(err)
	}

	// if the SetBalance does not error a lost update anomaly has occured
	return true, nil
}

/*
	An example of a Serialization Anomaly.
*/
func WriteSkew(ctx context.Context, pool *pgxpool.Pool, tx1, tx2 pgx.Tx) (bool, error) {
	defer tx1.Rollback(ctx)
	defer tx2.Rollback(ctx)
	// Tx1 reads Alice balance
	aliceBal := GetBalance(ctx, tx1, Alice)
	// Tx2 reads Bob balance
	bobBal := GetBalance(ctx, tx2, Bob)

	// Tx1 updates Bob balance which means Tx2's read is now incorrect
	err := SetBalance(ctx, tx1, Bob, aliceBal.Value+50)
	if err != nil {
		panic(err)
	}
	err = tx1.Commit(ctx)
	if err != nil {
		panic(err)
	}

	// Tx2 tries to write to the database but it's read is incorrect
	// this will fail for Serializable Isolation
	err = SetBalance(ctx, tx2, Alice, bobBal.Value+50)
	if err != nil {
		// This is only expected for Serializable Isolation
		return false, err
	}
	err = tx2.Commit(ctx)
	if err != nil {
		panic(err)
	}

	// if the SetBalance does not error a write skew anomaly has occured
	return true, nil
}

type ConnOrTx interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
}

func SetBalance(ctx context.Context, connOrTx ConnOrTx, name string, value int) error {
	_, err := connOrTx.Exec(ctx, "UPDATE dev.balances SET value = $1 WHERE name = $2", value, name)
	return err
}

func GetNegativeBalances(ctx context.Context, connOrTx ConnOrTx) []*Balance {
	var balances []*Balance
	err := pgxscan.Select(ctx, connOrTx, &balances, "SELECT value, name FROM dev.balances WHERE value < 0")
	if err != nil {
		panic(err)
	}

	return balances
}

func GetBalance(ctx context.Context, connOrTx ConnOrTx, name string) *Balance {
	var balance Balance
	err := pgxscan.Get(ctx, connOrTx, &balance, "SELECT value, name FROM dev.balances WHERE name = $1", name)
	if err != nil {
		panic(err)
	}

	return &balance
}
