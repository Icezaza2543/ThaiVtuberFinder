// Package sqlite is a deliberately small, parameterized wrapper over native SQLite.
// All access is serialized. Transactions hold the same mutex until commit/rollback.
package sqlite

/*
#cgo pkg-config: sqlite3
#include <sqlite3.h>
#include <stdlib.h>
static int bind_text(sqlite3_stmt *s, int i, const char *p, int n) {
 return sqlite3_bind_text(s, i, p, n, SQLITE_TRANSIENT);
}
*/
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

type DB struct {
	mu  sync.Mutex
	ptr *C.sqlite3
}
type Tx struct{ db *DB }

func Open(path string) (*DB, error) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	d := &DB{}
	if C.sqlite3_open_v2(p, &d.ptr, C.SQLITE_OPEN_READWRITE|C.SQLITE_OPEN_CREATE|C.SQLITE_OPEN_FULLMUTEX, nil) != C.SQLITE_OK {
		err := d.err()
		d.Close()
		return nil, err
	}
	C.sqlite3_busy_timeout(d.ptr, 5000)
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA foreign_keys=ON"} {
		if e := d.Exec(q); e != nil {
			d.Close()
			return nil, e
		}
	}
	return d, nil
}
func (d *DB) err() error {
	if d.ptr == nil {
		return errors.New("sqlite database closed")
	}
	return fmt.Errorf("sqlite: %s", C.GoString(C.sqlite3_errmsg(d.ptr)))
}
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr == nil {
		return nil
	}
	if C.sqlite3_close_v2(d.ptr) != C.SQLITE_OK {
		return d.err()
	}
	d.ptr = nil
	return nil
}
func (d *DB) Query(q string, args ...string) ([][]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.query(q, args...)
}
func (d *DB) Exec(q string, args ...string) error                { _, e := d.Query(q, args...); return e }
func (t *Tx) Query(q string, args ...string) ([][]string, error) { return t.db.query(q, args...) }
func (t *Tx) Exec(q string, args ...string) error                { _, e := t.Query(q, args...); return e }
func (d *DB) Transaction(fn func(*Tx) error) (err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err = d.query("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = d.query("ROLLBACK")
		}
	}()
	if err = fn(&Tx{db: d}); err != nil {
		return err
	}
	_, err = d.query("COMMIT")
	committed = err == nil
	return err
}
func (d *DB) query(q string, args ...string) ([][]string, error) {
	if d.ptr == nil {
		return nil, errors.New("sqlite database closed")
	}
	query := C.CString(q)
	defer C.free(unsafe.Pointer(query))
	var stmt *C.sqlite3_stmt
	var tail *C.char
	if C.sqlite3_prepare_v2(d.ptr, query, -1, &stmt, &tail) != C.SQLITE_OK {
		return nil, d.err()
	}
	if stmt == nil {
		return nil, errors.New("sqlite: empty statement")
	}
	defer C.sqlite3_finalize(stmt)
	if strings.TrimSpace(C.GoString(tail)) != "" {
		return nil, errors.New("sqlite: one statement per call required")
	}
	if int(C.sqlite3_bind_parameter_count(stmt)) != len(args) {
		return nil, errors.New("sqlite: parameter count mismatch")
	}
	for i, s := range args {
		p := C.CString(s)
		rc := C.bind_text(stmt, C.int(i+1), p, C.int(len(s)))
		C.free(unsafe.Pointer(p))
		if rc != C.SQLITE_OK {
			return nil, d.err()
		}
	}
	rows := [][]string{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, d.err()
		}
		n := int(C.sqlite3_column_count(stmt))
		row := make([]string, n)
		for i := 0; i < n; i++ {
			p := C.sqlite3_column_text(stmt, C.int(i))
			if p != nil {
				row[i] = C.GoStringN((*C.char)(unsafe.Pointer(p)), C.sqlite3_column_bytes(stmt, C.int(i)))
			}
		}
		rows = append(rows, row)
	}
}
