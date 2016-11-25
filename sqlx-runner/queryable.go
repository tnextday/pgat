package runner

import (
	"database/sql"
	"fmt"
	"regexp"

	"github.com/pkg/errors"

	"github.com/jmoiron/sqlx"
	"github.com/tnextday/pgat/dat"
)

// Queryable is an object that can be queried.
type Queryable struct {
	runner database
}

// WrapSqlxExt converts a sqlx.Ext to a *Queryable
func WrapSqlxExt(e sqlx.Ext) (*Queryable, error) {
	switch e := e.(type) {
	default:
		return nil, dat.NewError(fmt.Sprintf("unexpected type %T", e))
	case database:
		return &Queryable{e}, nil
	}
}

// SplitEx splits a string using a regex
// Idea from http://stackoverflow.com/a/14765076
func splitEx(text string, reg *regexp.Regexp) []string {
	indexes := reg.FindAllStringIndex(text, -1)
	laststart := 0
	result := make([]string, len(indexes)+1)
	for i, element := range indexes {
		result[i] = text[laststart:element[0]]
		laststart = element[1]
	}
	result[len(indexes)] = text[laststart:len(text)]
	return result
}

// ExecScript executes a script with multiple statements delimited by a separator ('GO')
func (q *Queryable) ExecScript(script string, args ...interface{}) error {
	statements := splitEx(script, reScriptSeparator)
	for _, sql := range statements {
		_, err := q.runner.Exec(sql, args...)
		if err != nil {
			return errors.Wrap(err, "SQL: "+sql)
		}
	}
	return nil
}

// Call creates a new CallBuilder for the given sproc and args.
func (q *Queryable) Call(sproc string, args ...interface{}) *dat.CallBuilder {
	b := dat.NewCallBuilder(sproc, args...)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// DeleteFrom creates a new DeleteBuilder for the given table.
func (q *Queryable) DeleteFrom(table string) *dat.DeleteBuilder {
	b := dat.NewDeleteBuilder(table)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// Exec executes a SQL query with optional arguments.
func (q *Queryable) Exec(cmd string, args ...interface{}) (*dat.Result, error) {
	var result sql.Result
	var err error

	if len(args) == 0 {
		result, err = q.runner.Exec(cmd)
	} else {
		result, err = q.runner.Exec(cmd, args...)
	}
	if err != nil {
		return nil, logSQLError(err, "Exec", cmd, args)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, logSQLError(err, "Exec", cmd, args)
	}
	return &dat.Result{RowsAffected: rowsAffected}, nil
}

// ExecBuilder executes the SQL in builder.
func (q *Queryable) ExecBuilder(b dat.Builder) error {
	sql, args, err := b.Interpolate()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		_, err = q.runner.Exec(sql)
	} else {
		_, err = q.runner.Exec(sql, args...)
	}
	if err != nil {
		return logSQLError(err, "ExecBuilder", sql, args)
	}
	return nil
}

// ExecMulti executes multiple SQL statements returning the number of
// statements executed, or the index at which an error occurred.
func (q *Queryable) ExecMulti(commands ...*dat.Expression) (int, error) {
	for i, cmd := range commands {
		_, err := q.runner.Exec(cmd.SQL, cmd.Args...)
		if err != nil {
			return i, err
		}
	}
	return len(commands), nil
}

// InsertInto creates a new InsertBuilder for the given table.
func (q *Queryable) InsertInto(table string) *dat.InsertBuilder {
	b := dat.NewInsertBuilder(table)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// Insect inserts or selects.
func (q *Queryable) Insect(table string) *dat.InsectBuilder {
	b := dat.NewInsectBuilder(table)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// Select creates a new SelectBuilder for the given columns.
func (q *Queryable) Select(columns ...string) *dat.SelectBuilder {
	b := dat.NewSelectBuilder(columns...)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// SelectDoc creates a new SelectBuilder for the given columns.
func (q *Queryable) SelectDoc(columns ...string) *dat.SelectDocBuilder {
	b := dat.NewSelectDocBuilder(columns...)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// SQL creates a new raw SQL builder.
func (q *Queryable) SQL(sql string, args ...interface{}) *dat.RawBuilder {
	b := dat.NewRawBuilder(sql, args...)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// Update creates a new UpdateBuilder for the given table.
func (q *Queryable) Update(table string) *dat.UpdateBuilder {
	b := dat.NewUpdateBuilder(table)
	b.Execer = NewExecer(q.runner, b)
	return b
}

// Upsert creates a new UpdateBuilder for the given table.
func (q *Queryable) Upsert(table string) *dat.UpsertBuilder {
	b := dat.NewUpsertBuilder(table)
	b.Execer = NewExecer(q.runner, b)
	return b
}
