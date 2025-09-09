package progress_bar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
	"golang.org/x/sync/errgroup"
)

type ProgressBar struct {
	sync.Mutex

	barNames []string
	poller   func(context.Context, func(string, int)) error

	err error
}

func NewProgressBar(poller func(context.Context, func(string, int)) error, bars ...string) *ProgressBar {
	return &ProgressBar{
		poller:   poller,
		barNames: bars,
	}
}

func (b *ProgressBar) Show() {
	group, ctx := errgroup.WithContext(context.Background())

	barPipes := make(map[string]chan int)

	for _, name := range b.barNames {
		barPipes[name] = make(chan int)
	}

	update := func(name string, p int) {
		if pipe, ok := barPipes[name]; ok {
			pipe <- p
		}
	}

	group.Go(func() error {
		defer func() {
			for _, pipe := range barPipes {
				close(pipe)
			}
		}()

		return b.poller(ctx, update)
	})

	group.Go(func() error {
		var wg sync.WaitGroup

		for _, name := range b.barNames {
			wg.Add(1)

			go b.renderer(name, barPipes[name], &wg)

			// This keeps the order of given bar names
			time.Sleep(time.Millisecond)
		}

		uiprogress.Start()
		defer func() {
			uiprogress.Stop()
			fmt.Println()
		}()

		wg.Wait()

		return nil
	})

	var err error

	defer func() {
		b.Lock()
		defer b.Unlock()

		b.err = err
	}()

	err = group.Wait()
}

func (b *ProgressBar) Err() error {
	b.Lock()
	defer b.Unlock()

	return b.err
}

func (b *ProgressBar) renderer(name string, pipe <-chan int, wg *sync.WaitGroup) {
	defer wg.Done()

	bar := uiprogress.AddBar(100).AppendCompleted()
	bar.Width = 50

	var status string

	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.Resize(fmt.Sprintf("%s: %*s", name, (32-len(name)), status), 35)
	})

	bar.Set(0)

	for p := range pipe {
		switch {
		case p == -1:
			status = "failed"
			bar.Set(bar.Current())
			return
		case p == 0:
			status = "waiting"
		case p >= 100:
			status = "completed"
			p = 100
		default:
			status = "syncing"
		}

		bar.Set(p)
	}
}
