package cbus

import (
	"fmt"
	"testing"
)

func TestHandlerNotFoundError_Error(t *testing.T) {
	command := intCommand(1)
	err := &HandlerNotFoundError{command}

	if err.Error() != "cbus: Handler not found for Command type cbus.intCommand" {
		t.Fatal()
	}
}

func TestIsHandlerNotFoundError(t *testing.T) {
	err := fmt.Errorf("error")
	if IsHandlerNotFoundError(err) {
		t.Fatal()
	}
	err = &HandlerNotFoundError{"error"}
	if !IsHandlerNotFoundError(err) {
		t.Fatal()
	}
}

func TestExecutionPanicError_Error(t *testing.T) {
	err := &ExecutionPanicError{
		Panic: 1,
	}

	if err.Error() != "cbus: panic while executing command 1" {
		t.Fatal(err.Error())
	}
}
