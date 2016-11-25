package runner

import (
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/jmoiron/sqlx"
	"github.com/tnextday/pgat/dat"
)

const (
	txPending = iota
	txCommitted
	txRollbacked
	txErred
)

// ErrTxRollbacked occurs when Commit() or Rollback() is called on a
// transaction that has already been rollbacked.
var ErrTxRollbacked = errors.New("Nested transaction already rollbacked")

// Tx is a transaction abstraction
type Tx struct {
	sync.Mutex
	*sqlx.Tx
	*Queryable
	IsRollbacked bool
	state        int
	stateStack   []int
}

// WrapSqlxTx creates a Tx from a sqlx.Tx
func WrapSqlxTx(tx *sqlx.Tx) *Tx {
	newtx := &Tx{Tx: tx, Queryable: &Queryable{tx}}
	if dat.Strict {
		time.AfterFunc(1*time.Minute, func() {
			if !newtx.IsRollbacked && newtx.state == txPending {
				panic("A database transaction was not closed!")
			}
		})
	}
	return newtx
}

// Begin creates a transaction for the given database
func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Beginx()
	if err != nil {
		if dat.Strict {
			Logger.Fatal("Could not create transaction")
		}
		return nil, Logger.Error("begin.error", err)
	}
	Logger.Debug("begin tx")
	return WrapSqlxTx(tx), nil
}

// Begin returns this transaction
func (tx *Tx) Begin() (*Tx, error) {
	tx.Lock()
	defer tx.Unlock()
	if tx.IsRollbacked {
		return nil, ErrTxRollbacked
	}

	Logger.Debug("begin nested tx")
	tx.pushState()
	return tx, nil
}

// Commit commits the transaction
func (tx *Tx) Commit() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.IsRollbacked {
		return Logger.Error("Cannot commit", ErrTxRollbacked)
	}

	if tx.state == txCommitted {
		return Logger.Error("Transaction has already been commited")
	}
	if tx.state == txRollbacked {
		return Logger.Error("Transaction has already been rollbacked")
	}

	if len(tx.stateStack) == 0 {
		err := tx.Tx.Commit()
		if err != nil {
			tx.state = txErred
			return Logger.Error("commit.error", err)
		}
	}

	Logger.Debug("commit")
	tx.state = txCommitted
	return nil
}

// Rollback cancels the transaction
func (tx *Tx) Rollback() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.IsRollbacked {
		return Logger.Error("Cannot rollback", ErrTxRollbacked)
	}
	if tx.state == txCommitted {
		return Logger.Error("Cannot rollback, transaction has already been commited")
	}

	// rollback is sent to the database even in nested state
	err := tx.Tx.Rollback()
	if err != nil {
		tx.state = txErred
		return Logger.Error("Unable to rollback", "err", err)
	}

	Logger.Debug("rollback")
	tx.state = txRollbacked
	tx.IsRollbacked = true
	return nil
}

// AutoCommit commits a transaction IF neither Commit or Rollback were called.
func (tx *Tx) AutoCommit() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.state == txRollbacked || tx.IsRollbacked {
		tx.popState()
		return nil
	}

	err := tx.Tx.Commit()
	if err != nil {
		tx.state = txErred
		if dat.Strict {
			log.Fatalf("Could not commit transaction: %s\n", err.Error())
		}
		tx.popState()
		return Logger.Error("transaction.AutoCommit.commit_error", err)
	}
	Logger.Debug("autocommit")
	tx.state = txCommitted
	tx.popState()
	return err
}

// AutoRollback rolls back transaction IF neither Commit or Rollback were called.
func (tx *Tx) AutoRollback() error {
	tx.Lock()
	defer tx.Unlock()

	if tx.IsRollbacked || tx.state == txCommitted {
		tx.popState()
		return nil
	}

	err := tx.Tx.Rollback()
	if err != nil {
		tx.state = txErred
		if dat.Strict {
			log.Fatalf("Could not rollback transaction: %s\n", err.Error())
		}
		tx.popState()
		return Logger.Error("transaction.AutoRollback.rollback_error", err)
	}
	Logger.Debug("autorollback")
	tx.state = txRollbacked
	tx.IsRollbacked = true
	tx.popState()
	return err
}

// Select creates a new SelectBuilder for the given columns.
// This disambiguates between Queryable.Select and sqlx's Select
func (tx *Tx) Select(columns ...string) *dat.SelectBuilder {
	return tx.Queryable.Select(columns...)
}

func (tx *Tx) pushState() {
	tx.stateStack = append(tx.stateStack, tx.state)
	tx.state = txPending
}

func (tx *Tx) popState() {
	if len(tx.stateStack) == 0 {
		return
	}

	var val int
	val, tx.stateStack = tx.stateStack[len(tx.stateStack)-1], tx.stateStack[:len(tx.stateStack)-1]
	tx.state = val
}
