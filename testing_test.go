package lumberjack

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// assert will log the given message if condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	tb.Helper()

	assertUp(tb, condition, 1, msg, v...)
}

// assertUp is like assert, but used inside helper functions, to ensure that
// the file and line number reported by failures corresponds to one or more
// levels up the stack.
func assertUp(tb testing.TB, condition bool, caller int, msg string, v ...interface{}) {
	tb.Helper()

	if !condition {
		_, file, line, _ := runtime.Caller(caller + 1)
		v = append([]interface{}{filepath.Base(file), line}, v...)
		tb.Logf("%s:%d: "+msg+"\n", v...)

		tb.FailNow()
	}
}

// equals tests that the two values are equal according to reflect.DeepEqual.
func equals(tb testing.TB, exp, act interface{}) {
	tb.Helper()

	equalsUp(tb, exp, act)
}

// equalsUp is like equals, but used inside helper functions, to ensure that the
// file and line number reported by failures corresponds to one or more levels
// up the stack.
func equalsUp(tb testing.TB, exp, act interface{}) {
	tb.Helper()

	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(2)
		tb.Logf("%s:%d: exp: %v (%T), got: %v (%T)\n",
			filepath.Base(file), line, exp, exp, act, act)
		tb.FailNow()
	}
}

// isNil reports a failure if the given value is not nil.  Note that values
// which cannot be nil will always fail this check.
func isNil(tb testing.TB, obtained interface{}) {
	tb.Helper()

	isNilUp(tb, obtained)
}

// isNilUp is like isNil, but used inside helper functions, to ensure that the
// file and line number reported by failures corresponds to one or more levels
// up the stack.
func isNilUp(tb testing.TB, obtained interface{}) {
	tb.Helper()

	if !_isNil(obtained) {
		_, file, line, _ := runtime.Caller(2)
		tb.Logf("%s:%d: expected nil, got: %v\n", filepath.Base(file), line, obtained)
		tb.FailNow()
	}
}

// notNil reports a failure if the given value is nil.
func notNil(tb testing.TB, obtained interface{}) {
	tb.Helper()

	notNilUp(tb, obtained, 1)
}

// notNilUp is like notNil, but used inside helper functions, to ensure that the
// file and line number reported by failures corresponds to one or more levels
// up the stack.
func notNilUp(tb testing.TB, obtained interface{}, caller int) {
	tb.Helper()

	if _isNil(obtained) {
		_, file, line, _ := runtime.Caller(caller + 1)
		tb.Logf("%s:%d: expected non-nil, got: %v\n", filepath.Base(file), line, obtained)
		tb.FailNow()
	}
}

// _isNil is a helper function for isNil and notNil, and should not be used
// directly.
func _isNil(obtained interface{}) bool {
	if obtained == nil {
		return true
	}

	switch v := reflect.ValueOf(obtained); v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}

	return false
}
