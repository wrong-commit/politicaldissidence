package jobs

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKickInvokesAllJobs(t *testing.T) {
	var count int32
	j1 := Func("one", func(Context) error {
		atomic.AddInt32(&count, 1)
		return nil
	})
	j2 := Func("two", func(Context) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	Kick(Context{}, j1, j2)

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&count) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Fatalf("expected 2 job runs, got %d", got)
	}
}

func TestKickLogsJobError(t *testing.T) {
	var mu sync.Mutex
	var logged string
	var isErr bool
	done := make(chan struct{})

	j := Func("failing", func(Context) error {
		return errors.New("boom")
	})

	Kick(Context{
		Log: func(msg string, isError bool) {
			mu.Lock()
			logged = msg
			isErr = isError
			mu.Unlock()
			close(done)
		},
	}, j)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error log")
	}

	mu.Lock()
	defer mu.Unlock()
	if !isErr {
		t.Fatal("expected error log")
	}
	if logged == "" || logged[:5] != "ERROR" {
		t.Fatalf("unexpected log: %q", logged)
	}
}

func TestFuncNameAndNil(t *testing.T) {
	j := Func("named", nil)
	if j.Name() != "named" {
		t.Fatalf("Name = %q", j.Name())
	}
	if err := j.Run(Context{}); err != nil {
		t.Fatalf("nil fn should no-op: %v", err)
	}
}

func TestNewWhoisOnAdd(t *testing.T) {
	var mp, dom int
	j := NewWhoisOnAdd(func(mpIndex, domainIdx int) {
		mp, dom = mpIndex, domainIdx
	})
	if j.Name() != "whois-on-add" {
		t.Fatalf("Name = %q", j.Name())
	}
	if err := j.Run(Context{MPIndex: 3, DomainIdx: 7}); err != nil {
		t.Fatal(err)
	}
	if mp != 3 || dom != 7 {
		t.Fatalf("got mp=%d dom=%d", mp, dom)
	}
	if err := NewWhoisOnAdd(nil).Run(Context{}); err != nil {
		t.Fatalf("nil run: %v", err)
	}
}
