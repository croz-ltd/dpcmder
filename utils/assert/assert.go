package assert

import (
	"reflect"
	"testing"
)

// DeepEqual compares given objects and fails test if objects are not deep equals.
func DeepEqual(t *testing.T, testName, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s should be: '%#v' but was: '%#v'.", testName, want, got)
	}
}