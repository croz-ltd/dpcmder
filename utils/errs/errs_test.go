package errs

import (
	"testing"
)

func TestError(t *testing.T) {
	testErr := Error("my-test-error")
	if testErr.Error() != "my-test-error" {
		t.Errorf("Error() produced is not 'my-test-error' but '%s'.", testErr.Error())
	}
}
