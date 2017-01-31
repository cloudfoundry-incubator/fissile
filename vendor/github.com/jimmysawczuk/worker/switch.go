package worker

import (
	"sync"
)

type lockSwitch struct {
	val  bool
	lock *sync.RWMutex
}

func (s *lockSwitch) init() {
	if s.lock == nil {
		s.lock = new(sync.RWMutex)
	}
}

func (s *lockSwitch) On() bool {
	s.init()

	s.lock.RLock()
	defer s.lock.RUnlock()

	r := s.val

	return r
}

func (s *lockSwitch) Toggle() {
	s.init()

	s.lock.Lock()
	defer s.lock.Unlock()

	s.val = !s.val
}

func (s *lockSwitch) Set(v bool) {
	s.init()

	s.lock.Lock()
	defer s.lock.Unlock()

	s.val = v
}
