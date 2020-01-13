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

func TestErrorf(t *testing.T) {
	testErr := Errorf("my-test-error '%s'.", "aha")
	if testErr.Error() != "my-test-error 'aha'." {
		t.Errorf("Error() produced is not 'my-test-error' but '%s'.", testErr.Error())
	}
}

func TestUnexpectedHTTPResponse(t *testing.T) {
	testErr := UnexpectedHTTPResponse{StatusCode: 444, Status: "Some error"}
	want := "UnexpectedHTTPResponse(444 'Some error')"
	if testErr.Error() != want {
		t.Errorf("Error string is not '%s' but '%s'.", want, testErr.Error())
	}
}
