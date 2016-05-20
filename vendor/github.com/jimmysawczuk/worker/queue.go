package worker

import (
	"sync"
)

// A Queue is an ordered list of Jobs.
type Queue struct {
	jobs []*Package
	lock *sync.RWMutex
}

// NewQueue returns a new, empty Queue.
func NewQueue() Queue {
	q := Queue{
		jobs: make([]*Package, 0),
		lock: new(sync.RWMutex),
	}

	return q
}

// Top returns the first Package in the Queue.
func (q *Queue) Top() *Package {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.jobs) > 0 {
		j := q.jobs[0]
		q.jobs = q.jobs[1:]
		return j
	}

	return nil
}

// Add adds a Package to the end of the Queue.
func (q *Queue) Add(j *Package) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.jobs = append(q.jobs, j)
}

// Prepend adds a Package to the front of the Queue.
func (q *Queue) Prepend(j *Package) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.jobs = append([]*Package{j}, q.jobs...)
}

// Len returns the length of the Queue.
func (q *Queue) Len() int {
	q.lock.RLock()
	defer q.lock.RUnlock()

	l := len(q.jobs)

	return l
}
