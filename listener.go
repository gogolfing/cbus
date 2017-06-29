package cbus

import "context"

//EventType is an enumeration of types of events that occur in a Command's execution
//lifecycle.
type EventType string

const (
	//Before denotes Events that are called after a Handler has been found for a
	//Command but before the Command's Handler is called.
	Before EventType = "Before"

	//AfterSuccess denotes Events that are called after a Command's Handler has
	//returned with a nil error.
	AfterSuccess EventType = "AfterSuccess"

	//AfterError denotes Events that are called after a Command's Handler has
	//returned with a non-nil error.
	AfterError EventType = "AfterError"

	//Complete denotes Events that are called after a Command's Handler has
	//returned regardless of successful or error completion.
	//The Complete Event is called after all prior After* Events have completed.
	Complete EventType = "Complete"
)

//Event is the type that is emitted during a Command's lifecycle.
type Event struct {
	//EventType is the type of the Event. This will designate what part of the
	//lifecycle a Command is in.
	EventType

	//Result is the possible result of that occurred during a Command Handler's execution.
	//Result will be nil on Before and AfterError Events.
	//It will be the result value that occurred for AfterSuccess and Complete Events
	//if there was a result.
	Result interface{}

	//Err is the possible error that occurred during a Command Handler's execution.
	//Err will be nil on Before and AfterSuccess Events.
	//It will be the error that occurred for AfterError and Complete Events if there
	//was an error.
	Err error

	//Command is the Command that is executing or has completed execution.
	Command
}

//Listener defines the contract for responding to an Event during a Command's lifecycle.
type Listener interface {
	OnEvent(ctx context.Context, event Event)
}

//ListenerFunc if a function implementation of a Listener.
type ListenerFunc func(ctx context.Context, event Event)

//OnEvent calls lf with ctx and event.
func (lf ListenerFunc) OnEvent(ctx context.Context, event Event) {
	lf(ctx, event)
}
