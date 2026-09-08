package jobs

import "fmt"

// Context carries domain-add details into background jobs.
type Context struct {
	MPIndex   int
	DomainIdx int
	Hostname  string
	Log       func(msg string, isError bool)
}

// Job is a unit of work kicked off when a domain is added.
type Job interface {
	Name() string
	Run(ctx Context) error
}

// Kick starts each job in its own goroutine (fire-and-forget).
func Kick(ctx Context, jobs ...Job) {
	for _, j := range jobs {
		j := j
		go func() {
			if err := j.Run(ctx); err != nil && ctx.Log != nil {
				ctx.Log(fmt.Sprintf("ERROR job %s: %v", j.Name(), err), true)
			}
		}()
	}
}

// Func adapts a plain function to Job without a dedicated type.
func Func(name string, fn func(Context) error) Job {
	return funcJob{name: name, fn: fn}
}

type funcJob struct {
	name string
	fn   func(Context) error
}

func (f funcJob) Name() string { return f.name }

func (f funcJob) Run(ctx Context) error {
	if f.fn == nil {
		return nil
	}
	return f.fn(ctx)
}
