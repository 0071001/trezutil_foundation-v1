package payment

import (
	"context"
	"sync"
)

// Locker provides mutual exclusion keyed by string.
type Locker interface {
	WithLock(ctx context.Context, key string, fn func(context.Context) error) error
}

// LocalLocker is an in-process Locker for tests and single-instance tools.
type LocalLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewLocalLocker creates a LocalLocker.
func NewLocalLocker() *LocalLocker {
	return &LocalLocker{locks: map[string]*sync.Mutex{}}
}

// WithLock runs fn while holding the mutex for key.
func (l *LocalLocker) WithLock(ctx context.Context, key string, fn func(context.Context) error) error {
	l.mu.Lock()
	m, ok := l.locks[key]
	if !ok {
		m = &sync.Mutex{}
		l.locks[key] = m
	}
	l.mu.Unlock()
	m.Lock()
	defer m.Unlock()
	return fn(ctx)
}
