package task

import (
	"context"
)

type Reporter interface {
	Send(context.Context, *TaskStat)
	SendProgress(context.Context, string, <-chan int)
}
