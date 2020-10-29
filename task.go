package gotask

import (
	"context"
)

type Task interface {
	Start(ctx context.Context) error
	Wait() error
}

type TaskReference interface {
	TaskGroupReference
	TaskID() string
}

type TaskGroupReference interface {
	GroupID() string
}

type Cancellable interface {
	Cancel()
}
