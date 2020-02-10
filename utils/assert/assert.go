// Package assert implements helper methods used in tests.
package assert

import (
	"bytes"
	"reflect"
	"testing"
)

// Equals compares given objects and fails test if objects are not equal.
func Equals(t *testing.T, testName string, got, want interface{}) {
	t.Helper()
	switch {
	case (got == nil) != (want == nil):
		t.Errorf("%s\nshould be: '%#v'\nbut was:   '%#v'.", testName, want, got)
	// case gotByte, gotOk := got.([]byte):
	default:
		gotBytes, gotOk := got.([]byte)
		wantBytes, wantOk := got.([]byte)
		switch {
		case gotOk && wantOk:
			if bytes.Compare(gotBytes, wantBytes) != 0 {
				t.Errorf("%s\nshould be: '%#v'\nbut was:   '%#v'.", testName, want, got)
			}
		case gotOk != wantOk:
			t.Errorf("%s\nshould be: '%#v'\nbut was:   '%#v'.", testName, want, got)
		default:
			if got != want {
				t.Errorf("%s\nshould be: '%#v'\nbut was:   '%#v'.", testName, want, got)
			}
		}
	}
}

// DeepEqual compares given objects and fails test if objects are not deep equals.
func DeepEqual(t *testing.T, testName string, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s\nshould be: '%v'\nbut was:   '%v'.", testName, want, got)
	}
}

// Nil fails test if given object is not null.
func Nil(t *testing.T, testName string, got interface{}) {
	t.Helper()
	switch got.(type) {
	case []byte:
		gotBytes, _ := got.([]byte)
		if gotBytes != nil {
			t.Errorf("%s\nwas not nil: '%#v'.", testName, got)
		}
	default:
		if got != nil {
			t.Errorf("%s\nwas not nil: '%#v'.", testName, got)
		}
	}
}

// NotNil fails test if given object is null.
func NotNil(t *testing.T, testName string, got interface{}) {
	t.Helper()
	if got == nil {
		t.Errorf("%s\nwas nil: '%#v'.", testName, got)
	}
}

// False fails test if given value is true.
func False(t *testing.T, testName string, got bool) {
	t.Helper()
	if got {
		t.Errorf("%s\nwas true: '%T'.", testName, got)
	}
}

// True fails test if given value is false.
func True(t *testing.T, testName string, got bool) {
	t.Helper()
	if !got {
		t.Errorf("%s\nwas false: '%T'.", testName, got)
	}
}
