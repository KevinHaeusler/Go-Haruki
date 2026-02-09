package session

import (
	"sync"
	"time"
)

type Session[T any] struct {
	Data      T
	ExpiresAt time.Time
}

type Store[T any] struct {
	mu       sync.RWMutex
	sessions map[string]*Session[T]
	ttl      time.Duration
}

func NewStore[T any](ttl time.Duration) *Store[T] {
	return &Store[T]{
		sessions: make(map[string]*Session[T]),
		ttl:      ttl,
	}
}

func (s *Store[T]) Get(userID string) *T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[userID]
	if !ok {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return &sess.Data
}

func (s *Store[T]) Set(userID string, data T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[userID] = &Session[T]{
		Data:      data,
		ExpiresAt: time.Now().Add(s.ttl),
	}
}

func (s *Store[T]) Clear(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, userID)
}

func (s *Store[T]) Touch(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[userID]; ok {
		sess.ExpiresAt = time.Now().Add(s.ttl)
	}
}
