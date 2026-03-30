package service

import (
	"sync"

	"github.com/mhwang-1/meadow-bubbletea/internal/store"
)

// Service consolidates all business logic for task management operations.
// It sits between API/bot callers and the store layer, providing validated,
// atomic operations. All write methods acquire the mutex; read methods do not
// (store reads are atomic via temp+rename).
type Service struct {
	store *store.Store
	mu    sync.Mutex
}

// New creates a new Service backed by the given store.
func New(s *store.Store) *Service {
	return &Service{store: s}
}
