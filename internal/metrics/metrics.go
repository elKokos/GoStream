package metrics

import "sync/atomic"

type Registry struct {
	HTTPRequests atomic.Uint64
}

func New() *Registry {
	return &Registry{}
}
