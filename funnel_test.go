package funnel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestRequestReturnsSuccessfulOutput(t *testing.T) {
	f := New(1, context.Background())
	out, err := f.Request(&Job{Command: "printf", Options: []string{"hello"}})
	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}
	if got, want := out.String(), "hello"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got := out.Stderr(); got != "" {
		t.Errorf("Stderr() = %q, want empty", got)
	}
	if got := out.Error(); got != nil {
		t.Errorf("Error() = %v, want nil", got)
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

func TestRequestRejectsWorkAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := New(1, ctx)

	out, err := f.Request(&Job{Command: "printf", Options: []string{"unexpected"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Request() error = %v, want context.Canceled", err)
	}
	if out != nil {
		t.Fatalf("Request() output = %#v, want nil", out)
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

func TestExternalWaitGroupTracksRunningRequest(t *testing.T) {
	var requests sync.WaitGroup
	f := NewFunnelWithWaitGroup(1, &requests, context.Background())
	marker := filepath.Join(t.TempDir(), "started")
	done := make(chan struct{})

	go func() {
		_, _ = f.Request(&Job{
			Command: "sh",
			Options: []string{"-c", "touch \"$1\"; sleep 0.1", "sh", marker},
		})
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("command did not start: %v", err)
	}
	requests.Wait()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WaitGroup completed before Request returned")
	}
}

func TestConstructorsRejectInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		new  func()
	}{
		{name: "zero parallelism", new: func() { New(0, context.Background()) }},
		{name: "negative parallelism", new: func() { New(-1, context.Background()) }},
		{name: "nil context", new: func() { New(1, nil) }},
		{name: "nil wait group", new: func() { NewFunnelWithWaitGroup(1, nil, context.Background()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("constructor did not panic")
				}
			}()
			tt.new()
		})
	}
}

func TestRequestRejectsNilJob(t *testing.T) {
	f := New(1, context.Background())
	defer func() {
		if recover() == nil {
			t.Fatal("Request(nil) did not panic")
		}
	}()
	f.Request(nil)
}
