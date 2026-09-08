package jobs

import (
	"fmt"
	"os/exec"
)

// Starter starts an external process. Injected for tests.
type Starter func() error

func defaultCalcStarter() error {
	return exec.Command("calc.exe").Start()
}

// NewCalcDemo returns a demo job that opens Windows Calculator.
// If start is nil, calc.exe is launched via exec.Command(...).Start().
func NewCalcDemo(start ...Starter) Job {
	var starter Starter = defaultCalcStarter
	if len(start) > 0 && start[0] != nil {
		starter = start[0]
	}
	return Func("calc-demo", func(ctx Context) error {
		if err := starter(); err != nil {
			return fmt.Errorf("start calc.exe: %w", err)
		}
		if ctx.Log != nil {
			ctx.Log(fmt.Sprintf("INFO calc-demo opened for domain %s", ctx.Hostname), false)
		}
		return nil
	})
}
