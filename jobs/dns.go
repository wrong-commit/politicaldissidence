package jobs

// NewDnsOnAdd wraps the UI DNS-on-add path as a Job.
// run receives the MP and domain indices from Context.
func NewDnsOnAdd(run func(mpIndex, domainIdx int)) Job {
	return Func("dns-on-add", func(ctx Context) error {
		if run == nil {
			return nil
		}
		run(ctx.MPIndex, ctx.DomainIdx)
		return nil
	})
}
