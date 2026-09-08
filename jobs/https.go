package jobs

// NewHttpsOnAdd wraps the UI HTTPS-on-add path as a Job.
// run receives the MP and domain indices from Context.
func NewHttpsOnAdd(run func(mpIndex, domainIdx int)) Job {
	return Func("https-on-add", func(ctx Context) error {
		if run == nil {
			return nil
		}
		run(ctx.MPIndex, ctx.DomainIdx)
		return nil
	})
}
