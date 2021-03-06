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
//		bus := &Bus{}
//
//		bus.Handle(&CreateUserCommand{}, HandlerFunc(func(ctx context.Context, command Command) (interface{}, error) {
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
//This package requires Go version 1.7 or higher because it uses the context package.
package cbus

import (
	"context"
	"reflect"
	"sync"
)

//Command is an empty interface that anything can implement and allows for
//executing arbitrary values on a Bus.
//Therefore, a Command is any defined type that get associated with Handler.
//The specific implementation of a Command can then carry the payload for the command
//to execute.
type Command interface{}

//Handler defines the contract for executing a Command within a context.Context.
//The result and err return parameters will be returned from Bus.Execute*() calls
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

//Bus is the Command Bus implementation.
//A Bus contains a one to one mapping from Command types to Handlers.
//The reflect.TypeOf interface is used as keys to map from Commands to Handlers.
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
//The zero value for Bus is fully functional.
//Type Bus is safe for use by multiple goroutines.
type Bus struct {
	//lock protects all other fields in Bus.
	lock      sync.RWMutex
	handlers  map[reflect.Type]Handler
	listeners map[EventType][]*commandListener
}

//Handle associates a Handler in b that will be called when a Command whose type
//equals command's type.
//Only one Handler is allowed per Command type. Any previously added Handlers
//with the same commandType will be overwritten.
//prev is the Handler previously associated with commandType if it exists.
func (b *Bus) Handle(command Command, handler Handler) (prev Handler) {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.putHandler(reflect.TypeOf(command), handler)
}

func (b *Bus) putHandler(commandType reflect.Type, handler Handler) Handler {
	prev := b.handlers[commandType]
	if b.handlers == nil {
		b.handlers = map[reflect.Type]Handler{}
	}
	b.handlers[commandType] = handler
	return prev
}

//Listen registers lis to be called for all Commands at the time in the
//Command lifecycle denoted by et.
//A value for et that is not documented in this package will never be called.
func (b *Bus) Listen(et EventType, lis Listener) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.appendListener(et, nil, lis)
}

//ListenCommand registers lis to be called for Commands of the same type as command
//at the time in the Command lifecycle denoted by et.
//A value for et that is not documented in this package will never be called.
func (b *Bus) ListenCommand(et EventType, command Command, lis Listener) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.appendListener(et, command, lis)
}

func (b *Bus) appendListener(et EventType, command Command, lis Listener) {
	if b.listeners == nil {
		b.listeners = map[EventType][]*commandListener{}
	}

	_, ok := b.listeners[et]
	if !ok {
		b.listeners[et] = []*commandListener{}
	}

	b.listeners[et] = append(
		b.listeners[et],
		&commandListener{Command: command, lis: lis},
	)
}

//RemoveHandler removes the Handler associated with Command's type and returns it.
//This is a no-op and returns nil if a Handler does not exist for command.
func (b *Bus) RemoveHandler(command Command) Handler {
	b.lock.Lock()
	defer b.lock.Unlock()

	commandType := reflect.TypeOf(command)
	handler := b.handlers[commandType]
	delete(b.handlers, commandType)

	return handler
}

//RemoveListener removes all Listeners that match lis (via ==) and et.
//The return value indicates if any Listeners were removed.
func (b *Bus) RemoveListener(et EventType, lis Listener) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	removed := b.removeListeners(
		et,
		func(_ Command, _lis Listener) bool {
			return _lis == lis
		},
	)
	return len(removed) > 0
}

//RemoveListenerCommand remove all Listeners that were registered with Command type
//equal to command's and et.
//It returns all removed Listeners.
func (b *Bus) RemoveListenerCommand(et EventType, command Command) []Listener {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.removeListeners(
		et,
		func(_command Command, _ Listener) bool {
			return reflect.TypeOf(_command) == reflect.TypeOf(command)
		},
	)
}

func (b *Bus) removeListeners(et EventType, doRemove func(Command, Listener) bool) []Listener {
	result := []Listener{}

	i := 0
	etComListeners := b.listeners[et]

	for i < len(etComListeners) {
		etCl := etComListeners[i]
		if doRemove(etCl.Command, etCl.lis) {
			etComListeners = append(etComListeners[:i], etComListeners[i+1:]...)
			result = append(result, etCl.lis)
		} else {
			i++
		}
	}

	if b.listeners != nil {
		b.listeners[et] = etComListeners
	}

	return result
}

//Execute is sugar for b.ExecuteContext(context.Background(), command).
func (b *Bus) Execute(command Command) (result interface{}, err error) {
	return b.ExecuteContext(context.Background(), command)
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

	handler, ok := b.handlers[reflect.TypeOf(command)]
	if !ok {
		return nil, &HandlerNotFoundError{command}
	}

	return b.execute(ctx, command, handler)
}

func (b *Bus) execute(ctx context.Context, command Command, handler Handler) (interface{}, error) {
	done := make(chan *executePayload)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- &executePayload{nil, &ExecutionPanicError{r}}
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
		listener.onEvent(ctx, Event{
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
