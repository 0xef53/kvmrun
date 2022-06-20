package services

import (
	"sync"

	"github.com/0xef53/kvmrun/internal/grpcserver"
)

var pool = struct {
	sync.Mutex
	services []grpcserver.Registration
}{}

func Register(s grpcserver.Registration) {
	pool.Lock()
	defer pool.Unlock()

	pool.services = append(pool.services, s)
}

func Services() []grpcserver.Registration {
	pool.Lock()
	defer pool.Unlock()

	return pool.services
}
