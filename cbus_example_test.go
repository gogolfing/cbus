package cbus

import (
	"context"
	"testing"
	"time"
)

type User struct {
	Name string
}

type CreateUserCommand struct {
	Name string
}

func Test(t *testing.T) {
	bus := &Bus{}

	bus.Handle(&CreateUserCommand{}, HandlerFunc(func(ctx context.Context, command Command) (interface{}, error) {
		cuc := command.(*CreateUserCommand)

		user := &User{
			Name: cuc.Name,
		}
		return user, nil
	}))

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	result, err := bus.ExecuteContext(
		ctx,
		&CreateUserCommand{"Mr. Foo Bar"},
	)

	if err != nil {
		t.Fail()
	}

	if result.(*User).Name != "Mr. Foo Bar" {
		t.Fail()
	}
}
