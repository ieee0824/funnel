package funnel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRequestCapturesOutputAndError(t *testing.T) {
	f := New(1, context.Background())
	out, err := f.Request(&Job{
		Command: "sh",
		Options: []string{"-c", "printf stdout; printf stderr >&2; exit 7"},
	})
	if err == nil {
		t.Fatal("Request() error = nil, want command failure")
	}
	if got, want := out.Stdout(), "stdout"; got != want {
		t.Errorf("Stdout() = %q, want %q", got, want)
	}
	if got, want := out.Stderr(), "stderr"; got != want {
		t.Errorf("Stderr() = %q, want %q", got, want)
	}
	if !errors.Is(out.Error(), err) {
		t.Errorf("Output.Error() = %v, want %v", out.Error(), err)
	}
}

func TestRequestHonorsParallelLimit(t *testing.T) {
	f := New(1, context.Background())
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.Request(&Job{Command: "sleep", Options: []string{"0.1"}}); err != nil {
				t.Errorf("Request() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if elapsed := time.Since(start); elapsed < 250*time.Millisecond {
		t.Fatalf("three serialized commands completed in %v", elapsed)
	}
}

func TestRequestCancelsRunningCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := New(1, ctx)
	done := make(chan error, 1)
	go func() {
		_, err := f.Request(&Job{Command: "sleep", Options: []string{"10"}})
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Request() error = nil after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("running command was not canceled")
	}
}

func TestJobCanBeReusedConcurrently(t *testing.T) {
	f := New(4, context.Background())
	job := &Job{Command: "printf", Options: []string{"ok"}}
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := f.Request(job)
			if err != nil {
				t.Errorf("Request() error = %v", err)
				return
			}
			if got := out.Stdout(); got != "ok" {
				t.Errorf("Stdout() = %q, want %q", got, "ok")
			}
		}()
	}
	wg.Wait()
}
