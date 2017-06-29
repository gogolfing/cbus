package cbus

import "fmt"

//HandlerNotFoundError occurs when a Handler has not been registered for a Command's type.
type HandlerNotFoundError struct {
	//Command is the Command that a Handler was not found for.
	Command
}

//Error is the error implementation.
func (e *HandlerNotFoundError) Error() string {
	return fmt.Sprintf("cbus: Handler not found for Command type %T", e.Command)
}

//IsHandlerNotFoundError determines whether or not err is of type *HandlerNotFoundError.
func IsHandlerNotFoundError(err error) bool {
	_, ok := err.(*HandlerNotFoundError)
	return ok
}

//ExecutionPanicError occurs when a Handler or Listener panics during execution.
type ExecutionPanicError struct {
	//Panic is the value received from recover() if not nil.
	Panic interface{}
}

//Error is the error implementation.
func (e *ExecutionPanicError) Error() string {
	return fmt.Sprintf("cbus: panic while executing command %v", e.Panic)
}
