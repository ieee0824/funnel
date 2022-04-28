package funnel

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"

	"golang.org/x/sync/semaphore"
)

type Job struct {
	Command string
	Options []string
	output  chan *Output
}

type Output struct {
	outStr    string
	outErrStr string
	err       error
}

func (o *Output) String() string {
	return o.outStr
}

func (o *Output) Error() error {
	return o.err
}

func New(maxParallelProcessNum int, ctx context.Context) *Funnel {
	impl := &Funnel{
		request:  make(chan *Job, 1024),
		wg:       new(sync.WaitGroup),
		ctx:      ctx,
		weighted: semaphore.NewWeighted(int64(maxParallelProcessNum)),
	}
	go impl.run()
	return impl
}

func NewFunnelWithWaitGroup(maxParallelProcessNum int, wg *sync.WaitGroup, ctx context.Context) *Funnel {
	impl := &Funnel{
		request:  make(chan *Job, 1024),
		wg:       wg,
		ctx:      ctx,
		weighted: semaphore.NewWeighted(int64(maxParallelProcessNum)),
	}
	go impl.run()
	return impl
}

type Funnel struct {
	request  chan *Job
	ctx      context.Context
	wg       *sync.WaitGroup
	stop     bool
	weighted *semaphore.Weighted
}

func (impl *Funnel) Wg() *sync.WaitGroup {
	return impl.wg
}

func (impl *Funnel) run() {
	for {
		select {
		case job := <-impl.request:

			go func() {
				defer impl.weighted.Release(1)
				defer impl.wg.Done()
				stdout := new(bytes.Buffer)
				stderr := new(bytes.Buffer)

				cmd := exec.Command(
					job.Command,
					job.Options...,
				)

				cmd.Stdout = stdout
				cmd.Stderr = stderr

				err := cmd.Run()
				if err != nil {
					job.output <- &Output{
						outStr:    stdout.String(),
						outErrStr: stderr.String(),
						err:       err,
					}
					return
				}

				job.output <- &Output{
					outStr:    stdout.String(),
					outErrStr: stderr.String(),
				}
			}()

		case <-impl.ctx.Done():
			impl.stop = true
		}

	}
}

func (impl *Funnel) Request(job *Job) (string, error) {
	if impl.stop {
		return "", fmt.Errorf("acceptance has stopped")
	}
	if err := impl.weighted.Acquire(impl.ctx, 1); err != nil {
		return "", err
	}
	impl.wg.Add(1)
	job.output = make(chan *Output, 1)
	impl.request <- job

	out := <-job.output
	close(job.output)
	if out.err != nil {
		return "", out.err
	}

	return out.outStr, nil
}
