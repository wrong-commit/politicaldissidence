package jobs

// NewWhoisOnAdd wraps the UI WHOIS-on-add path as a Job.
// run receives the MP and domain indices from Context.
func NewWhoisOnAdd(run func(mpIndex, domainIdx int)) Job {
	return Func("whois-on-add", func(ctx Context) error {
		if run == nil {
			return nil
		}
		run(ctx.MPIndex, ctx.DomainIdx)
		return nil
	})
}
