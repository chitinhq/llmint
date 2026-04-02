package llmint

// Chain composes zero or more Middleware into a single Middleware.
// Middleware are applied so that the first argument is the outermost layer —
// the same convention used by net/http middleware stacks.
//
// Example:
//
//	p := Chain(logging, rateLimit, cache)(base)
//	// Call path: logging → rateLimit → cache → base
func Chain(mw ...Middleware) Middleware {
	return func(next Provider) Provider {
		// Apply in reverse so that mw[0] ends up as the outermost wrapper.
		for i := len(mw) - 1; i >= 0; i-- {
			next = mw[i](next)
		}
		return next
	}
}
