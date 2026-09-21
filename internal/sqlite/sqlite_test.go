package sqlite

import (
	"errors"
	"testing"
)

func TestBoundParametersAndRollback(t *testing.T) {
	db, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.Exec("CREATE TABLE t(v TEXT UNIQUE)"); e != nil {
		t.Fatal(e)
	}
	text := "ไทย ' ; DROP TABLE t; -- \x00 end"
	if e = db.Exec("INSERT INTO t VALUES (?)", text); e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query("SELECT v FROM t")
	if e != nil || len(rows) != 1 || rows[0][0] != text {
		t.Fatalf("%v %#v", e, rows)
	}
	e = db.Transaction(func(tx *Tx) error {
		if e := tx.Exec("INSERT INTO t VALUES (?)", "rolled back"); e != nil {
			return e
		}
		return errors.New("abort")
	})
	if e == nil {
		t.Fatal("expected abort")
	}
	rows, _ = db.Query("SELECT v FROM t")
	if len(rows) != 1 {
		t.Fatal("rollback failed")
	}
}
func TestClosedDB(t *testing.T) {
	db, e := Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	if db.Exec("SELECT 1") == nil {
		t.Fatal("closed accepted")
	}
}
