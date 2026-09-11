package jobs

// NewRegistrarOnAdd wraps the UI registrar-on-add path as a Job.
func NewRegistrarOnAdd(run func(mpIndex, domainIdx int)) Job {
	return Func("registrar-on-add", func(ctx Context) error {
		if run == nil {
			return nil
		}
		run(ctx.MPIndex, ctx.DomainIdx)
		return nil
	})
}
