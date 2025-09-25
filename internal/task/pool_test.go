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

	"github.com/0xef53/kvmrun/internal/task/classifiers"
	test_utils "github.com/0xef53/kvmrun/internal/task/internal/testing"
	"github.com/0xef53/kvmrun/internal/task/metadata"
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

func TestPool_ConcurrentTasks(t *testing.T) {
	pool := NewPool()

	tryStart := func(tgt poolTest_target, lifetime time.Duration, mustOK bool) {
		tid := strconv.FormatUint(uint64(rand.Uint32()), 16)

		task := poolTest_dummyTask{
			GenericTask: new(GenericTask),
			targets:     tgt,
			id:          tid,
			lifetime:    lifetime,
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

func TestPool_TaskWaiting(t *testing.T) {
	pool := NewPool()

	tgt := poolTest_target{"machine_eve": modeAny}

	for i := 0; i < 2; i++ {
		task := poolTest_dummyTask{
			GenericTask:      new(GenericTask),
			targets:          tgt,
			id:               "id-1234567890",
			lifetime:         2,
			SleepBeforeStart: true,
		}

		tid, err := pool.StartTask(context.Background(), &task, nil)
		if err != nil {
			t.Fatal(poolTest_Format(nil, err, tgt))
		}

		pool.Wait(tid)
	}
}

func TestPool_TaskContextCanceling(t *testing.T) {
	pool := NewPool()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()

	task := poolTest_dummyTask{
		GenericTask: new(GenericTask),
		id:          "id-1234567890",
		lifetime:    3,
	}

	tid, err := pool.StartTask(ctx, &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	pool.Wait(tid)

	if !errors.Is(pool.Err(tid), context.DeadlineExceeded) {
		t.Fatal(poolTest_Format(context.DeadlineExceeded, pool.Err(tid), nil))
	}
}

func TestPool_TaskCanceling(t *testing.T) {
	pool := NewPool()

	task := poolTest_dummyTask{
		GenericTask: new(GenericTask),
		id:          "id-1234567890",
		lifetime:    4,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	go func() {
		time.Sleep(2 * time.Second)

		pool.Cancel(tid)
	}()

	pool.Wait(tid)

	if !errors.Is(pool.Err(tid), ErrTaskInterrupted) {
		t.Fatal(poolTest_Format(ErrTaskInterrupted, pool.Err(tid), nil))
	}
}

func TestPool_BeforeStartFunctionFailure(t *testing.T) {
	pool := NewPool()

	task := poolTest_dummyTask{
		GenericTask:             new(GenericTask),
		id:                      "id-1234567890",
		FailBeforeStartFunction: true,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)

	if err != test_utils.ErrSuccessfullyFailed {
		t.Fatal(poolTest_Format(test_utils.ErrSuccessfullyFailed, err, nil))
	}

	pool.Wait(tid)
}

func TestPool_OnSuccessFunctionFailure(t *testing.T) {
	pool := NewPool()

	task := poolTest_dummyTask{
		GenericTask:           new(GenericTask),
		id:                    "id-1234567890",
		FailOnSuccessFunction: true,
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	pool.Wait(tid)

	if !errors.Is(pool.Err(tid), test_utils.ErrSuccessfullyFailed) {
		t.Fatal(poolTest_Format(test_utils.ErrSuccessfullyFailed, pool.Err(tid), nil))
	}
}

func TestPool_Closing(t *testing.T) {
	pool := NewPool()

	start := func(idx int) (poolTest_target, error) {
		tgt := poolTest_target{"machine_vm" + strconv.Itoa(idx): modeAny}
		_, err := pool.StartTask(
			context.Background(),
			&poolTest_dummyTask{
				GenericTask: new(GenericTask),
				targets:     tgt,
				id:          "id-" + strconv.Itoa(idx),
				lifetime:    time.Duration(rand.Intn(5)),
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

func TestPool_TaskInfoExtracting(t *testing.T) {
	pool := NewPool()

	task := poolTest_dummyTask{
		GenericTask: new(GenericTask),
	}

	tid, err := pool.StartTask(context.Background(), &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	info, ok := InfoFromContext(task.Ctx())
	if info == nil {
		t.Fatal(poolTest_Format(true, ok, nil))
	}

	pool.Wait(tid)
}

func TestPool_TaskWithMetadata(t *testing.T) {
	pool := NewPool()

	task := poolTest_dummyTask{
		GenericTask: new(GenericTask),
	}

	type testMeta struct {
		Name string
	}

	md := testMeta{"some meta"}

	ctx := metadata.AppendToContext(context.Background(), &md)

	tid, err := pool.StartTask(ctx, &task, nil)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	if a, _ := metadata.FromContext(task.Ctx()); a != nil {
		if v, ok := a.(*testMeta); !ok {
			t.Fatal(poolTest_Format(md, v, nil))
		}
	} else {
		t.Fatal(poolTest_Format(md, a, nil))
	}

	pool.Wait(tid)
}

func TestPool_ClassifierRegistartion(t *testing.T) {
	pool := NewPool()

	clsA := classifiers.NewUniqueLabelClassifier()

	if _, err := pool.RegisterClassifier(clsA, "unique-labels"); err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	if _, err := pool.RegisterClassifier(clsA, "single-labels", "unique-labels"); !errors.Is(err, ErrRegistrationFailed) {
		t.Fatal(poolTest_Format(ErrRegistrationFailed, err, nil))
	}

	clsB := classifiers.NewGroupLabelClassifier()

	if _, err := pool.RegisterClassifier(clsB, "group-labels", "x-group-labels"); err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	if _, err := pool.RegisterClassifier(clsB, "x-group-labels"); !errors.Is(err, ErrRegistrationFailed) {
		t.Fatal(poolTest_Format(ErrRegistrationFailed, err, nil))
	}
}

func TestPool_TaskWithClassifier(t *testing.T) {
	pool := NewPool()

	cls := classifiers.NewUniqueLabelClassifier()

	if _, err := pool.RegisterClassifier(cls, "unique-labels"); err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	opts := []TaskOption{
		&TaskClassifierDefinition{
			Name: "unique-labels",
			Opts: &classifiers.UniqueLabelOptions{Label: "task"},
		},
	}

	task := poolTest_dummyTask{
		GenericTask: new(GenericTask),
	}

	_, err := pool.StartTask(context.Background(), &task, nil, opts...)
	if err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}
}

func TestPool_TaskWithUnknownClassifier(t *testing.T) {
	pool := NewPool()

	cls := classifiers.NewUniqueLabelClassifier()

	if _, err := pool.RegisterClassifier(cls, "unique-labels"); err != nil {
		t.Fatal(poolTest_Format(nil, err, nil))
	}

	opts := []TaskOption{
		&TaskClassifierDefinition{
			Name: "non-existent-classifier-name",
			Opts: &classifiers.UniqueLabelOptions{Label: "task"},
		},
	}

	task := poolTest_dummyTask{
		GenericTask: new(GenericTask),
	}

	_, err := pool.StartTask(context.Background(), &task, nil, opts...)
	if !errors.Is(err, ErrAssignmentFailed) {
		t.Fatal(poolTest_Format(ErrAssignmentFailed, err, nil))
	}
}
