package worker

import (
	"sync"
)

// Map holds a lookup table for Job Packages by ID.
type Map struct {
	jobs map[int64]*Package
	lock *sync.RWMutex
}

// NewMap returns a new empty Map
func NewMap() Map {
	m := Map{
		jobs: make(map[int64]*Package),
		lock: new(sync.RWMutex),
	}

	return m
}

// Set puts a provided Package into the Map, properly indexed by ID
func (m *Map) Set(val *Package) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.jobs[val.ID] = val
}

// Get returns the *Package at the location given by the ID provided.
func (m *Map) Get(id int64) *Package {
	m.lock.Lock()
	defer m.lock.Unlock()

	val := m.jobs[id]

	return val
}
