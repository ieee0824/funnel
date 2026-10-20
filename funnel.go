package funnel

import (
	"bytes"
	"context"
	"os/exec"
	"sync"

	"golang.org/x/sync/semaphore"
)

type Job struct {
	Command string
	Options []string
}

type Output struct {
	outStr    string
	outErrStr string
	err       error
}

func (o *Output) String() string {
	return o.outStr
}

func (o *Output) Stdout() string {
	return o.outStr
}

func (o *Output) Stderr() string {
	return o.outErrStr
}

func (o *Output) Error() error {
	return o.err
}

func New(maxParallelProcessNum int, ctx context.Context) *Funnel {
	return NewFunnelWithWaitGroup(maxParallelProcessNum, new(sync.WaitGroup), ctx)
}

func NewFunnelWithWaitGroup(maxParallelProcessNum int, wg *sync.WaitGroup, ctx context.Context) *Funnel {
	if maxParallelProcessNum <= 0 {
		panic("funnel: maxParallelProcessNum must be positive")
	}
	if wg == nil {
		panic("funnel: nil WaitGroup")
	}
	if ctx == nil {
		panic("funnel: nil Context")
	}

	return &Funnel{
		wg:       wg,
		ctx:      ctx,
		weighted: semaphore.NewWeighted(int64(maxParallelProcessNum)),
	}
}

type Funnel struct {
	ctx      context.Context
	wg       *sync.WaitGroup
	weighted *semaphore.Weighted
}

func (impl *Funnel) Wg() *sync.WaitGroup {
	return impl.wg
}

func (impl *Funnel) Request(job *Job) (*Output, error) {
	if job == nil {
		panic("funnel: nil Job")
	}

	if err := impl.weighted.Acquire(impl.ctx, 1); err != nil {
		return nil, err
	}
	defer impl.weighted.Release(1)

	impl.wg.Add(1)
	defer impl.wg.Done()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(impl.ctx, job.Command, job.Options...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()

	out := &Output{
		outStr:    stdout.String(),
		outErrStr: stderr.String(),
		err:       err,
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
