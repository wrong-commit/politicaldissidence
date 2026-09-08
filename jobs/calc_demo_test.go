package jobs

import (
	"errors"
	"sync"
	"testing"
)

func TestNewCalcDemoNameAndStart(t *testing.T) {
	var started bool
	j := NewCalcDemo(func() error {
		started = true
		return nil
	})
	if j.Name() != "calc-demo" {
		t.Fatalf("Name = %q", j.Name())
	}

	var mu sync.Mutex
	var info string
	err := j.Run(Context{
		Hostname: "example.com",
		Log: func(msg string, isError bool) {
			mu.Lock()
			info = msg
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("starter was not called")
	}
	mu.Lock()
	defer mu.Unlock()
	if info == "" {
		t.Fatal("expected info log")
	}
}

func TestNewCalcDemoStartError(t *testing.T) {
	j := NewCalcDemo(func() error {
		return errors.New("no calc")
	})
	err := j.Run(Context{Hostname: "x.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewCalcDemoDefaultStarterNonNil(t *testing.T) {
	j := NewCalcDemo()
	if j.Name() != "calc-demo" {
		t.Fatalf("Name = %q", j.Name())
	}
	// Do not call Run — that would launch calc.exe.
}
