package cbus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestErrHandlerNotFound_Error(t *testing.T) {
	command := intCommand(1)
	err := &ErrHandlerNotFound{command}

	if err.Error() != "cbus: Handler not found for Command type cbus.intCommand" {
		t.Fatal()
	}
}

func TestIsErrHandlerNotFound(t *testing.T) {
	err := fmt.Errorf("error")
	if IsErrHandlerNotFound(err) {
		t.Fatal()
	}
	err = &ErrHandlerNotFound{"error"}
	if !IsErrHandlerNotFound(err) {
		t.Fatal()
	}
}

func TestBus_Handle(t *testing.T) {
	bus := &Bus{}

	prev := bus.Handle("1", intHandler(1))
	if prev != nil {
		t.Fail()
	}
	if bus.handlers[reflect.TypeOf("1")] != intHandler(1) {
		t.Fail()
	}

	prev = bus.Handle("1", intHandler(2))
	if prev != intHandler(1) {
		t.Fail()
	}
	if bus.handlers[reflect.TypeOf("1")] != intHandler(2) {
		t.Fail()
	}
}

func TestBus_Listen(t *testing.T) {
	bus := &Bus{}

	bus.Listen(Before, intListener(1))
	if !reflect.DeepEqual(bus.listeners[Before], []Listener{intListener(1)}) {
		t.Fail()
	}

	bus.Listen(Before, intListener(2))
	if !reflect.DeepEqual(bus.listeners[Before], []Listener{intListener(1), intListener(2)}) {
		t.Fail()
	}
}

func TestBus_RemoveHandler(t *testing.T) {
	bus := &Bus{}

	bus.Handle("handler", intHandler(1))

	if handler := bus.RemoveHandler(struct{}{}); handler != nil {
		t.Fail()
	}

	handler := bus.RemoveHandler("handler")

	if handler != intHandler(1) || len(bus.handlers) != 0 {
		t.Fail()
	}
}

func TestBus_RemoveListener(t *testing.T) {
	tests := []struct {
		eventType EventType
		listeners []Listener

		removeType EventType
		toRemove   Listener

		resultListeners []Listener
		found           bool
	}{
		{
			Before,
			nil,
			Before,
			nil,
			nil,
			false,
		},
		{
			Before,
			[]Listener{intListener(1)},
			Before,
			intListener(1),
			[]Listener{},
			true,
		},
		{
			Before,
			[]Listener{intListener(1)},
			Complete,
			intListener(1),
			[]Listener{intListener(1)},
			false,
		},
		{
			Before,
			[]Listener{intListener(1), intListener(2)},
			Before,
			intListener(1),
			[]Listener{intListener(2)},
			true,
		},
		{
			Before,
			[]Listener{intListener(1), intListener(2)},
			Before,
			intListener(2),
			[]Listener{intListener(1)},
			true,
		},
	}
	for index, test := range tests {
		bus := &Bus{}
		for _, listener := range test.listeners {
			bus.Listen(test.eventType, listener)
		}
		found := bus.RemoveListener(test.removeType, test.toRemove)

		if found != test.found || !reflect.DeepEqual(bus.listeners[test.eventType], test.resultListeners) {
			t.Errorf(
				"%v bus.RemoveListener(%v, %v) = %v, %v WANT %v, %v",
				index,
				test.removeType,
				test.toRemove,
				found,
				bus.listeners[test.eventType],
				test.found,
				test.resultListeners,
			)
		}
	}
}

func TestBus_Execute_allEventsGetCalledAndReturnResultIsFromHandler(t *testing.T) {
	bus := &Bus{}

	command := intCommand(1)

	before, afterSuccess, complete := false, false, false

	bus.Listen(Before, ListenerFunc(func(ctx context.Context, event Event) {
		if before || afterSuccess || complete {
			t.Fail()
		}
		if event.Result != nil {
			t.Fail()
		}
		if event.Err != nil {
			t.Fail()
		}
		before = true
	}))

	bus.Listen(AfterSuccess, ListenerFunc(func(ctx context.Context, event Event) {
		if !before || afterSuccess || complete {
			t.Fail()
		}
		if event.Result != "this is the result" {
			t.Fail()
		}
		if event.Err != nil {
			t.Fail()
		}
		afterSuccess = true
	}))

	bus.Listen(Complete, ListenerFunc(func(ctx context.Context, event Event) {
		if !before || !afterSuccess || complete {
			t.Fail()
		}
		if event.Result != "this is the result" {
			t.Fail()
		}
		if event.Err != nil {
			t.Fail()
		}
		complete = true
	}))

	bus.Handle(command, HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
		if cmd != command {
			t.Fail()
		}
		return "this is the result", nil
	}))

	result, err := bus.Execute(command)

	if result.(string) != "this is the result" || err != nil {
		t.Fail()
	}

	if !before || !afterSuccess || !complete {
		t.Fail()
	}
}

func TestBus_Execute_afterErrorEventListenerIsCalledForError(t *testing.T) {
	bus := &Bus{}

	command := intCommand(1)

	afterError, complete := false, false

	bus.Listen(AfterError, ListenerFunc(func(ctx context.Context, event Event) {
		if afterError || complete {
			t.Fail()
		}
		if event.Result != nil {
			t.Fail()
		}
		if event.Err == nil {
			t.Fail()
		}
		afterError = true
	}))

	bus.Listen(Complete, ListenerFunc(func(ctx context.Context, event Event) {
		if !afterError || complete {
			t.Fail()
		}
		if event.Result != nil {
			t.Fail()
		}
		if event.Err == nil {
			t.Fail()
		}
		complete = true
	}))

	bus.Handle(command, HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
		if cmd != command {
			t.Fail()
		}
		return nil, errors.New("this is the error")
	}))

	result, err := bus.Execute(command)

	if result != nil || err.Error() != "this is the error" {
		t.Fail()
	}

	if !afterError || !complete {
		t.Fail()
	}
}

func TestBus_Execute_errorHandlerNotFound(t *testing.T) {
	bus := &Bus{}

	command := intCommand(1)

	result, err := bus.Execute(command)

	if result != nil || !IsErrHandlerNotFound(err) {
		t.Fail()
	}
}

func TestBus_Execute_panicError(t *testing.T) {
	bus := &Bus{}

	command := intCommand(1)

	bus.Handle(command, HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
		panic("panic value")
	}))

	result, err := bus.Execute(command)

	if result != nil {
		t.Fail()
	}
	if eep := err.(*ErrExecutePanic); eep.Panic != "panic value" {
		t.Fail()
	}
}

func TestBus_ExecuteContext_errorsWithCancelledContext(t *testing.T) {
	bus := &Bus{}

	command := intCommand(1)

	bus.Handle(command, HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
		return "something we wont get later", nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10))
	defer cancel()

	result, err := bus.ExecuteContext(ctx, command)

	if result != nil || err != context.DeadlineExceeded {
		t.Fail()
	}
}

type intHandler int

func (ih intHandler) Handle(ctx context.Context, command Command) (interface{}, error) {
	return int(ih), nil
}

type intListener int

func (il intListener) OnEvent(ctx context.Context, event Event) {
}

type intCommand int

func (ic intCommand) Type() string {
	return fmt.Sprintf("%v", int(ic))
}

type panicHandler string

func (pc panicHandler) Type() string {
	return "panic"
}
