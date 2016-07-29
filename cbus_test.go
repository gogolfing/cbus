package cbus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	bus := New()

	if bus == nil || bus.lock == nil || bus.handlers == nil || bus.listeners == nil {
		t.Fail()
	}
}

func TestBus_Handle(t *testing.T) {
	bus := New()

	bus.Handle("1", intHandler(1))
	if bus.handlers["1"] != intHandler(1) {
		t.Fail()
	}

	bus.Handle("1", intHandler(2))
	if bus.handlers["1"] != intHandler(2) {
		t.Fail()
	}
}

func TestBus_Listen(t *testing.T) {
	bus := New()

	bus.Listen(Before, intListener(1))
	if !reflect.DeepEqual(bus.listeners[Before], []Listener{intListener(1)}) {
		t.Fail()
	}

	bus.Listen(Before, intListener(2))
	if !reflect.DeepEqual(bus.listeners[Before], []Listener{intListener(1), intListener(2)}) {
		t.Fail()
	}
}

func TestBus_Execute_allEventsGetCalledAndReturnResultIsFromHandler(t *testing.T) {
	bus := New()

	command := intCommand(1)

	before, afterSuccess, complete := false, false, false

	bus.Listen(Before, ListenerFunc(func(ctx context.Context, event Event) {
		if before || afterSuccess || complete {
			t.Fail()
		}
		before = true
	}))

	bus.Listen(AfterSuccess, ListenerFunc(func(ctx context.Context, event Event) {
		if !before || afterSuccess || complete {
			t.Fail()
		}
		afterSuccess = true
	}))

	bus.Listen(Complete, ListenerFunc(func(ctx context.Context, event Event) {
		if !before || !afterSuccess || complete {
			t.Fail()
		}
		complete = true
	}))

	bus.Handle("1", HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
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
	bus := New()

	command := intCommand(1)

	afterError, complete := false, false

	bus.Listen(AfterError, ListenerFunc(func(ctx context.Context, event Event) {
		if afterError || complete {
			t.Fail()
		}
		afterError = true
	}))

	bus.Listen(Complete, ListenerFunc(func(ctx context.Context, event Event) {
		if !afterError || complete {
			t.Fail()
		}
		complete = true
	}))

	bus.Handle("1", HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
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
	bus := New()

	command := intCommand(1)

	result, err := bus.Execute(command)

	if result != nil || err != ErrHandlerNotFound {
		t.Fail()
	}
}

func TestBus_ExecuteContext_errorsWithCancelledContext(t *testing.T) {
	bus := New()

	command := intCommand(1)

	bus.Handle("1", HandlerFunc(func(ctx context.Context, cmd Command) (interface{}, error) {
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
