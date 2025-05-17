package task

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

var errSuccessfullyFailed = errors.New("function is failed")

type DummyTask struct {
	*GenericTask

	id      string
	timeout time.Duration

	SleepBeforeStart        bool
	FailBeforeStartFunction bool
	FailOnSuccessFunction   bool
}

func (t *DummyTask) GetKey() string { return t.id }

func (t *DummyTask) BeforeStart(a interface{}) error {
	if t.SleepBeforeStart {
		time.Sleep(t.timeout * time.Second)
	}

	if t.FailBeforeStartFunction {
		return errSuccessfullyFailed
	}

	return nil
}

func (t *DummyTask) OnSuccess() error {
	if t.FailOnSuccessFunction {
		return errSuccessfullyFailed
	}

	return nil
}

func (t *DummyTask) Main() error {
	if t.timeout > 0 {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-time.After(t.timeout * time.Second):
		}
	}

	return nil
}

func resultStr(want, got error, id string) string {
	return fmt.Sprintf("got unexpected result:\n\twant:\t%v\n\tgot:\t%v\n\tid:\t%s", want, got, id)
}

func TestConcurrentTasks(t *testing.T) {
	pool := NewPool(4)

	tryStart := func(id string, timeout time.Duration, mustOK bool) {
		task := DummyTask{new(GenericTask), id, timeout, false, false, false}

		_, err := pool.StartTask(context.Background(), &task, nil)

		if mustOK {
			if err != nil {
				t.Fatal(resultStr(nil, err, id))
			}
		} else {
			if _, ok := err.(*TaskAlreadyRunningError); !ok {
				t.Fatal(resultStr(&TaskAlreadyRunningError{Key: id}, err, id))
			}
		}
	}

	// We run these tasks just to prepare the pool
	basicIDs := []string{
		"u221:vm123:20060102:",
		"u345:vm325:20060102:aabbccdd",
		"u876:vm224::",
		"u555:::",
	}

	for _, id := range basicIDs {
		tryStart(id, 3, true)
	}

	// And now we will check for collisions between them and new tasks
	for _, id := range []string{"u221:::", "u221:vm123::", "u221:vm123:20060102:", "u221:vm123:20060102:suffix"} {
		tryStart(id, 0, false)
	}

	for _, id := range []string{"u345:vm123::", "u345:vm325:20060103:", "u345:vm325:20060102:suffix"} {
		tryStart(id, 0, true)
	}

	for _, id := range []string{"u876:::", "u876:vm224::"} {
		tryStart(id, 0, false)
	}

	for _, id := range []string{"u876:vm225:20060102:", "u876:vm225:20060103:suffix"} {
		tryStart(id, 0, true)
	}

	for _, id := range []string{"u555:::", "u555:vm123::", "u555:vm124:20060102:", "u555:vm124:20060103:suffix"} {
		tryStart(id, 0, false)
	}
}

func TestTaskWaiting(t *testing.T) {
	pool := NewPool(4)

	for i := 0; i < 2; i++ {
		task := DummyTask{new(GenericTask), "u202:::", 3, true, false, false}
		id, err := pool.StartTask(context.Background(), &task, nil)
		if err != nil {
			t.Fatal(resultStr(nil, err, id))
		}
		pool.Wait(id)
	}
}

func TestTaskCanceling(t *testing.T) {
	pool := NewPool(4)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()

	task := DummyTask{new(GenericTask), "u232:::", 3, false, false, false}

	id, err := pool.StartTask(ctx, &task, nil)
	if err != nil {
		t.Fatal(resultStr(nil, err, id))
	}

	pool.Wait(id)

	if pool.Err(id) != context.DeadlineExceeded {
		t.Fatal(resultStr(context.DeadlineExceeded, pool.Err(id), id))
	}
}

func TestBeforeStartFunctionFailure(t *testing.T) {
	pool := NewPool(4)

	task := DummyTask{new(GenericTask), "u262:::", 0, true, true, false}

	id, err := pool.StartTask(context.Background(), &task, nil)
	if err != errSuccessfullyFailed {
		t.Fatal(resultStr(errSuccessfullyFailed, err, id))
	}

	pool.Wait(id)
}

func TestOnSuccessFunctionFailure(t *testing.T) {
	pool := NewPool(4)

	task := DummyTask{new(GenericTask), "u282:::", 0, false, false, true}

	id, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(resultStr(nil, err, id))
	}

	pool.Wait(id)

	if pool.Err(id) != errSuccessfullyFailed {
		t.Fatal(resultStr(errSuccessfullyFailed, pool.Err(id), id))
	}
}

func TestPoolClosing(t *testing.T) {
	pool := NewPool(4)

	rand.Seed(time.Now().UnixNano())

	start := func(idx int) (string, error) {
		return pool.StartTask(
			context.Background(),
			&DummyTask{
				new(GenericTask),
				strconv.Itoa(idx) + ":::",
				time.Duration(rand.Intn(5)),
				false,
				false,
				false,
			},
			nil,
		)
	}

	for i := 0; i <= 10; i++ {
		if id, err := start(i); err != nil {
			t.Fatal(resultStr(nil, err, id))
		}
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		pool.WaitAndClosePool()
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal(resultStr(nil, fmt.Errorf("pool closing timeout (currently running: %d)", len(pool.List())), "<nil>"))
	}

	if id, err := start(5000); err != ErrPoolClosed {
		t.Fatal(resultStr(ErrPoolClosed, err, id))
	}
}
