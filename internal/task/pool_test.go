package task

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	test_utils "github.com/0xef53/kvmrun/internal/task/internal/testing"
)

const (
	modeChangePropertyName OperationMode = 1 << (16 - 1 - iota)
	modeChangePropertyDiskName
	modeChangePropertyDiskSize
	modeChangePropertyNetName
	modeChangePropertyNetLink
	modePowerUp
	modePowerDown
	modePowerCycle

	modeAny                = ^OperationMode(0)
	modeChangePropertyDisk = modeChangePropertyDiskName | modeChangePropertyDiskSize
	modeChangePropertyNet  = modeChangePropertyNetName | modeChangePropertyNetLink
	modeChangeProperties   = modeChangePropertyDisk | modeChangePropertyNet
	modePowerManagement    = modePowerUp | modePowerDown | modePowerCycle
)

type poolTest_dummyTask struct {
	*GenericTask

	targets map[string]OperationMode

	id       string
	lifetime time.Duration

	SleepBeforeStart        bool
	FailBeforeStartFunction bool
	FailOnSuccessFunction   bool
}

func (t *poolTest_dummyTask) Targets() map[string]OperationMode { return t.targets }

func (t *poolTest_dummyTask) BeforeStart(a interface{}) error {
	if t.SleepBeforeStart {
		time.Sleep(t.lifetime * time.Second)
	}

	if t.FailBeforeStartFunction {
		return test_utils.ErrSuccessfullyFailed
	}

	return nil
}

func (t *poolTest_dummyTask) OnSuccess() error {
	if t.FailOnSuccessFunction {
		return test_utils.ErrSuccessfullyFailed
	}

	return nil
}

func (t *poolTest_dummyTask) Main() error {
	if t.lifetime > 0 {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-time.After(t.lifetime * time.Second):
		}
	}

	return nil
}

type poolTest_target map[string]OperationMode

func (t poolTest_target) String() string {
	pairs := make([]string, 0, len(t))

	for k, v := range t {
		pairs = append(pairs, fmt.Sprintf("%s:%b", k, v))
	}

	return strings.Join(pairs, ", ")
}

func poolTest_Format(want, got interface{}, tgt poolTest_target) string {
	return fmt.Sprintf("%s\n\ttarget:\t%s\n", test_utils.FormatResultString(want, got), tgt)
}

func TestConcurrentTasks(t *testing.T) {
	pool := NewPool()

	tryStart := func(tgt poolTest_target, lifetime time.Duration, mustOK bool) {
		tid := strconv.FormatUint(uint64(rand.Uint32()), 16)

		task := poolTest_dummyTask{
			GenericTask:             new(GenericTask),
			targets:                 tgt,
			id:                      tid,
			lifetime:                lifetime,
			SleepBeforeStart:        false,
			FailBeforeStartFunction: false,
			FailOnSuccessFunction:   false,
		}

		_, err := pool.StartTask(context.Background(), &task, nil)

		if mustOK {
			if err != nil {
				t.Fatal(poolTest_Format(nil, err, tgt))
			}
		} else {
			if _, ok := err.(*ConcurrentRunningError); !ok {
				t.Fatal(poolTest_Format(&ConcurrentRunningError{"DummyTask", tgt}, err, tgt))
			}
		}
	}

	// We run some tasks with these targets just to prepare the pool
	basicTargets := []poolTest_target{
		{
			// blocks any other actions with virt.machine
			"machine_alice": modeChangeProperties | modePowerManagement,
		},
		{
			// blocks only the disk actions
			"machine_bob": modeChangePropertyDisk,
			// and blocks any actions with specified disk
			"machine_bob:disk_A": modeAny,
		},
		{
			// blocks only the net actions
			"machine_carol": modeChangePropertyNet,
			// and blocks any actions with specified netif
			"machine_carol:net_A": modeAny,
		},
	}

	for _, tgt := range basicTargets {
		tryStart(tgt, 3, true)
	}

	// And now we will check for collisions between them and new tasks
	aliceTargets := []poolTest_target{
		{"machine_alice": modePowerDown},         // false
		{"machine_alice": modeChangePropertyNet}, // false
	}
	for _, tgt := range aliceTargets {
		tryStart(tgt, 0, false)
	}

	bobTargets := []poolTest_target{
		{"machine_bob": modeChangePropertyNet, "machine_bob:net_A": modeAny},
		{"machine_bob": modeChangePropertyName},
		{"machine_bob:disk_B": modeChangePropertyDiskSize},
	}
	for _, tgt := range bobTargets {
		tryStart(tgt, 0, true)
	}

	carolTargets := []poolTest_target{
		{"machine_carol": modeChangeProperties | modePowerDown},
		{"machine_carol:net_A": modeChangePropertyNetLink},
	}
	for _, tgt := range carolTargets {
		tryStart(tgt, 0, false)
	}
}

func TestTaskWaiting(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_eve": modeAny}

	for i := 0; i < 2; i++ {
		task := poolTest_dummyTask{
			GenericTask:             new(GenericTask),
			targets:                 tgt,
			id:                      "id-1234567890",
			lifetime:                3,
			SleepBeforeStart:        true,
			FailBeforeStartFunction: false,
			FailOnSuccessFunction:   false,
		}

		tid, err := pool.StartTask(context.Background(), &task, nil)
		if err != nil {
			t.Fatal(poolTest_Format(nil, err, tgt))
		}

		pool.Wait(tid)
	}
}

func TestTaskContextCanceling(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_frank": modeAny}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		targets:                 tgt,
		id:                      "id-1234567890",
		lifetime:                3,
		SleepBeforeStart:        false,
		FailBeforeStartFunction: false,
		FailOnSuccessFunction:   false,
	}

	tid, err := pool.StartTask(ctx, &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, tgt))
	}

	pool.Wait(tid)

	if !errors.Is(pool.Err(tid), context.DeadlineExceeded) {
		t.Fatal(poolTest_Format(context.DeadlineExceeded, pool.Err(tid), tgt))
	}
}

func TestTaskCanceling(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_frank": modeAny}

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		targets:                 tgt,
		id:                      "id-1234567890",
		lifetime:                4,
		SleepBeforeStart:        false,
		FailBeforeStartFunction: false,
		FailOnSuccessFunction:   false,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, tgt))
	}

	go func() {
		time.Sleep(2 * time.Second)

		pool.Cancel(tid)
	}()

	pool.Wait(tid)

	if !errors.Is(pool.Err(tid), ErrTaskInterrupted) {
		t.Fatal(poolTest_Format(ErrTaskInterrupted, pool.Err(tid), tgt))
	}
}

func TestBeforeStartFunctionFailure(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_grace": modeAny}

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		targets:                 tgt,
		id:                      "id-1234567890",
		lifetime:                0,
		SleepBeforeStart:        true,
		FailBeforeStartFunction: true,
		FailOnSuccessFunction:   false,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)

	if err != test_utils.ErrSuccessfullyFailed {
		t.Fatal(poolTest_Format(test_utils.ErrSuccessfullyFailed, err, tgt))
	}

	pool.Wait(tid)
}

func TestOnSuccessFunctionFailure(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_grace": modeAny}

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		targets:                 tgt,
		id:                      "id-1234567890",
		lifetime:                0,
		SleepBeforeStart:        false,
		FailBeforeStartFunction: false,
		FailOnSuccessFunction:   true,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, tgt))
	}

	pool.Wait(tid)

	if !errors.Is(pool.Err(tid), test_utils.ErrSuccessfullyFailed) {
		t.Fatal(poolTest_Format(test_utils.ErrSuccessfullyFailed, pool.Err(tid), tgt))
	}
}

func TestPoolClosing(t *testing.T) {
	pool := NewPool()

	start := func(idx int) (poolTest_target, error) {
		tgt := poolTest_target{"machine_vm" + strconv.Itoa(idx): modeAny}
		_, err := pool.StartTask(
			context.Background(),
			&poolTest_dummyTask{
				GenericTask:             new(GenericTask),
				targets:                 tgt,
				id:                      "id-" + strconv.Itoa(idx),
				lifetime:                time.Duration(rand.Intn(5)),
				SleepBeforeStart:        false,
				FailBeforeStartFunction: false,
				FailOnSuccessFunction:   false,
			},
			nil,
		)

		return tgt, err
	}

	for i := 0; i <= 10; i++ {
		if tgt, err := start(i); err != nil {
			t.Fatal(poolTest_Format(nil, err, tgt))
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
		t.Fatal(poolTest_Format(nil, fmt.Errorf("pool closing timeout (currently running: %d)", len(pool.List())), nil))
	}

	if tgt, err := start(5000); !errors.Is(err, ErrPoolClosed) {
		t.Fatal(poolTest_Format(ErrPoolClosed, err, tgt))
	}
}

func TestTaskInfoExtracting(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_alice": modeAny}

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		id:                      "1234567890",
		targets:                 tgt,
		lifetime:                1,
		SleepBeforeStart:        true,
		FailBeforeStartFunction: false,
		FailOnSuccessFunction:   false,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, tgt))
	}

	info, ok := InfoFromContext(task.Ctx())
	if info == nil {
		t.Fatal(poolTest_Format(true, ok, tgt))
	}

	pool.Wait(tid)
}

func TestTaskMetadata(t *testing.T) { // TODO
	pool := NewPool()

	tgt := poolTest_target{"machine_alice": modeAny}

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		id:                      "1234567890",
		targets:                 tgt,
		lifetime:                1,
		SleepBeforeStart:        true,
		FailBeforeStartFunction: false,
		FailOnSuccessFunction:   false,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, tgt))
	}

	info, ok := InfoFromContext(task.Ctx())
	if info == nil {
		t.Fatal(poolTest_Format(true, ok, tgt))
	}

	pool.Wait(tid)
}

// Регистрация классификатора по десколькими именами
// * попрбобовать также повторно зарегистрировать
// * регистрация таски в нормальном существующем
// * регситрация в несуществующем

// Дерегистрация классификатора
// * попробовать также удалить повторно
// * регистрация таски в несуществующем
// * удаление классификатора с таской
