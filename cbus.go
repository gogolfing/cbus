package cbus

import (
	"context"
	"errors"
	"sync"
)

var ErrHandlerNotFound = errors.New("cbus: handler not found")

type Bus struct {
	lock *sync.RWMutex

	handlers map[string]Handler

	listeners map[EventType][]Listener
}

func New() *Bus {
	return &Bus{
		lock: &sync.RWMutex{},

		handlers: map[string]Handler{},

		listeners: map[EventType][]Listener{},
	}
}

func (b *Bus) Handle(commandType string, handler Handler) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.handlers[commandType] = handler
}

func (b *Bus) Listen(et EventType, l Listener) {
	b.lock.Lock()
	defer b.lock.Unlock()

	_, ok := b.listeners[et]
	if !ok {
		b.listeners[et] = []Listener{}
	}
	b.listeners[et] = append(b.listeners[et], l)
}

func (b *Bus) RemoveHandler(commandType string) Handler {
	b.lock.Lock()
	defer b.lock.Unlock()

	handler := b.handlers[commandType]
	delete(b.handlers, commandType)

	return handler
}

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

func (b *Bus) Execute(command Command) (result interface{}, err error) {
	return b.ExecuteContext(context.TODO(), command)
}

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
		b.dispatchEvent(ctx, Before, command)

		result, err := handler.Handle(ctx, command)

		if err == nil {
			b.dispatchEvent(ctx, AfterSuccess, command)
		} else {
			b.dispatchEvent(ctx, AfterError, command)
		}

		b.dispatchEvent(ctx, Complete, command)

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

func (b *Bus) dispatchEvent(ctx context.Context, et EventType, command Command) {
	listeners := b.listeners[et]

	for _, listener := range listeners {
		listener.OnEvent(ctx, Event{
			EventType: et,
			Command:   command,
		})
	}
}

type executePayload struct {
	result interface{}
	err    error
}

type Handler interface {
	Handle(ctx context.Context, command Command) (result interface{}, err error)
}

type HandlerFunc func(ctx context.Context, command Command) (result interface{}, err error)

func (hf HandlerFunc) Handle(ctx context.Context, command Command) (result interface{}, err error) {
	return hf(ctx, command)
}

type Command interface {
	Type() string
}

type EventType string

const (
	Before       EventType = "Before"
	AfterSuccess           = "AfterSuccess"
	AfterError             = "AfterError"
	Complete               = "Complete"
)

type Event struct {
	EventType

	Command
}

type Listener interface {
	OnEvent(ctx context.Context, event Event)
}

type ListenerFunc func(ctx context.Context, event Event)

func (lf ListenerFunc) OnEvent(ctx context.Context, event Event) {
	lf(ctx, event)
}
