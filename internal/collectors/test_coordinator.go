package collectors

import (
	"sync"
)

// TestCoordinator ensures only one internet performance test runs at a time
// across all backends to prevent network bandwidth contention
type TestCoordinator struct {
	mu sync.Mutex
}

var globalCoordinator = &TestCoordinator{}

// GetCoordinator returns the global test coordinator
func GetCoordinator() *TestCoordinator {
	return globalCoordinator
}

// TryLock attempts to acquire the lock for running a test.
// Returns true if the lock was acquired, false if another test is currently running.
func (tc *TestCoordinator) TryLock() bool {
	return tc.mu.TryLock()
}

// Unlock releases the lock after a test completes.
func (tc *TestCoordinator) Unlock() {
	tc.mu.Unlock()
}
