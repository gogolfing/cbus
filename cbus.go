//Package cbus provides a Command Bus implementation that allows client code
//to execute arbitrary commands with a given handler.
//
//Example
//
//	type User struct {
//		Name string
//	}
//
//	type CreateUserCommand struct {
//		Name string
//	}
//
//	func (cuc *CreateUserCommand) Type() string {
//		return "CreateUser"
//	}
//
//	func main() {
//		bus := New()
//
//		bus.Handle("CreateUser", HandlerFunc(func(ctx context.Context, command Command) (interface{}, error) {
//			user := &User{
//				Name: command.(*CreateUserCommand).Name,
//			}
//			return user, nil
//		}))
//
//		ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
//		result, _ := bus.ExecuteContext(
//			ctx,
//			&CreateUserCommand{"Mr. Foo Bar"},
//		)
//
//		fmt.Println(result.(*User).Name) //Mr. Foo Bar
//	}
//
//This package requires Go version 1.7 or higher because it uses the newly added
//context package.
package cbus

import (
	"context"
	"errors"
	"sync"
)

//ErrHandlerNotFound is the value returned from *Bus.Execute*() if a Handler
//does not exist for a command's Type.
var ErrHandlerNotFound = errors.New("cbus: handler not found")

//ErrExecutePanic is an error that occurs if the executing goroutine for
//a Command's Event Listeners or Handler panics.
type ErrExecutePanic struct {
	//Panic is the value returned from recover() if not nil.
	Panic interface{}
}

//Error is the error implementation for e.
func (e *ErrExecutePanic) Error() string {
	return "cbus: panic while executing command"
}

//Bus is the Command Bus implementation.
//A Bus contains a one to one mapping from Command types to Handlers.
//It additionally contains Listeners that are called during specific steps during
//a Command's execution.
//
//All Command Handlers and Event Listeners are called from a Bus in a newly spawned
//goroutine per Command execution.
//The Before Listeners are called just before the Command's Handler is called.
//The Command Handler will not be called until the optional Listeners have returned.
//After the Command Handler returns, either the AfterSuccess or AfterError Listeners
//will be called depending on the existence of err returned from the Handler.
//The Complete Listeners are called after the After* events regardless of successful
//or errored results from the Handler.
//All registered Listeners are called in the order they were added via Listen().
//
//Bus is safe for use by multiple goroutines.
type Bus struct {
	//lock protects all other fields in Bus.
	lock      *sync.RWMutex
	handlers  map[string]Handler
	listeners map[EventType][]Listener
}

//New creates a new Bus with no Handlers or Listeners.
func New() *Bus {
	return &Bus{
		lock:      &sync.RWMutex{},
		handlers:  map[string]Handler{},
		listeners: map[EventType][]Listener{},
	}
}

//Handle adds a Handler to b that will be called when a Command whose Type()
//equals commandType is executed on b.
//Only one Handler is allowed per Command type. Any previously added Handlers
//with the same commandType will be overwritten.
//prev is the Handler previously associated with commandType if it exists.
func (b *Bus) Handle(commandType string, handler Handler) (prev Handler) {
	b.lock.Lock()
	defer b.lock.Unlock()

	prev = b.handlers[commandType]

	b.handlers[commandType] = handler

	return prev
}

//Listen registers a Listener to be called for all Commands at the time in the
//command lifecycle denoted by et.
//A value for et that is not documented in this package will never be called.
func (b *Bus) Listen(et EventType, l Listener) {
	b.lock.Lock()
	defer b.lock.Unlock()

	_, ok := b.listeners[et]
	if !ok {
		b.listeners[et] = []Listener{}
	}
	b.listeners[et] = append(b.listeners[et], l)
}

//RemoveHandler removes the Handler associated with commandType and returns it.
//This is a no-op and returns nil if a Handler does not exist for commandType.
func (b *Bus) RemoveHandler(commandType string) Handler {
	b.lock.Lock()
	defer b.lock.Unlock()

	handler := b.handlers[commandType]
	delete(b.handlers, commandType)

	return handler
}

//RemoveListener removes all Listeners that match l (via ==) and et.
//The return value indicates if any Listeners were removed.
func (b *Bus) RemoveListener(et EventType, l Listener) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	i, found := 0, false
	for i < len(b.listeners[et]) {
		if b.listeners[et][i] == l {
			b.listeners[et] = append(b.listeners[et][:i], b.listeners[et][i+1:]...)
			found = true
		} else {
			i++
		}
	}
	return found
}

//Execute is sugar for b.ExecuteContext(context.TODO(), command).
func (b *Bus) Execute(command Command) (result interface{}, err error) {
	return b.ExecuteContext(context.TODO(), command)
}

//ExecuteContext attempts to find a Handler for command's Type().
//If a Handler is not found, then ErrHandlerNotFound is returned immediately.
//If a Handler is found, then a new goroutine is spawned and all registered Before
//Listeners are called, followed by command's Handler, finally followed by all
//registered After* and Complete Listeners.
//
//If ctx.Done() is closed before the event Listeners and command Handler complete,
//then ctx.Err() is returned with a nil result.
func (b *Bus) ExecuteContext(ctx context.Context, command Command) (result interface{}, err error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	handler, ok := b.handlers[command.Type()]
	if !ok {
		return nil, ErrHandlerNotFound
	}

	return b.execute(ctx, command, handler)
}

func (b *Bus) execute(ctx context.Context, command Command, handler Handler) (interface{}, error) {
	done := make(chan *executePayload)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- &executePayload{nil, &ErrExecutePanic{r}}
			}
		}()

		b.dispatchEvent(ctx, Before, command, nil, nil)

		result, err := handler.Handle(ctx, command)

		if err == nil {
			b.dispatchEvent(ctx, AfterSuccess, command, result, err)
		} else {
			b.dispatchEvent(ctx, AfterError, command, result, err)
		}

		b.dispatchEvent(ctx, Complete, command, result, err)

		done <- &executePayload{result, err}
	}()

	payload := &executePayload{}
	select {
	case <-ctx.Done():
		payload.err = ctx.Err()
	case payload = <-done:
	}

	return payload.result, payload.err
}

func (b *Bus) dispatchEvent(ctx context.Context, et EventType, command Command, result interface{}, err error) {
	listeners := b.listeners[et]

	for _, listener := range listeners {
		listener.OnEvent(ctx, Event{
			EventType: et,
			Result:    result,
			Err:       err,
			Command:   command,
		})
	}
}

type executePayload struct {
	result interface{}
	err    error
}

//Handler defines the contract for executing a Command within a context.Context.
//The result and err return parameters will be returned from *Bus.Execute*() calls
//which allows Command executors to know the results of the Command's execution.
type Handler interface {
	Handle(ctx context.Context, command Command) (result interface{}, err error)
}

//HandlerFunc is a function definition for a Handler.
type HandlerFunc func(ctx context.Context, command Command) (result interface{}, err error)

//Handle calls hf with ctx and command.
func (hf HandlerFunc) Handle(ctx context.Context, command Command) (result interface{}, err error) {
	return hf(ctx, command)
}

//Command provides an interface for executing arbitrary values on a *Bus.
//A Command need only provide a type string that determines which Handler gets called
//during Command execution.
//The specific implementation of a Command can then carry the payload for the command
//to execute.
type Command interface {
	Type() string
}

//EventType is an enumeration of types of events that occur in a Command's execution
//lifecycle.
type EventType string

const (
	//Before denotes Events that are called after a Handler has been found for a
	//Command but before the Command's Handler is called.
	Before EventType = "Before"

	//AfterSuccess denotes Events that are called after a Command's Handler has
	//returned with a nil error.
	AfterSuccess = "AfterSuccess"

	//AfterError denotes Events that are called after a Command's Handler has
	//returned with a non-nil error.
	AfterError = "AfterError"

	//Complete denotes Events that are called after a Command's Handler has
	//returned regardless of successful or error completion.
	//The Complete Event is called after all prior After* Events have completed.
	Complete = "Complete"
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
