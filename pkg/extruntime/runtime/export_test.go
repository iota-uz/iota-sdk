package runtime

// RouteAudits maps "METHOD /full/path" of every management route to the audit
// action a refused request is recorded under.
func RouteAudits() map[string]string {
	out := map[string]string{}
	for _, rt := range routes {
		out[rt.method+" "+APIPrefix+openAPIPath(rt.path)] = rt.audit
	}
	return out
}

// RouteAlternatives maps "METHOD /full/path" to the documented alternative to
// the route's permission ("" when there is none).
func RouteAlternatives() map[string]string {
	out := map[string]string{}
	for _, rt := range routes {
		out[rt.method+" "+APIPrefix+openAPIPath(rt.path)] = rt.alt
	}
	return out
}

// WithEmptySlices exposes the encoder's normalization.
func WithEmptySlices(v any) any { return withEmptySlices(v) }
