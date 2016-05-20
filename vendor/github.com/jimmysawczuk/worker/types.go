package worker

import (
	"sync"
)

// ExitCode is a code for how the Worker should exit.
type ExitCode int

const (
	// ExitNormally indicates that the Worker is exiting without error.
	ExitNormally ExitCode = 0

	// ExitWhenDone indicates that the Worker should finish all of its Jobs first, then exit.
	ExitWhenDone = 4
)

// An Event is fired when Jobs change within a Worker.
type Event int

const (
	jobAdded Event = 1 << iota
	jobStarted
	jobFinished

	// JobAdded is fired when a Job (Package) is added to the Worker's queue.
	JobAdded Event = 1 << iota

	// JobStarted is fired when a Job (Package) begins executing.
	JobStarted

	// JobFinished is fired when a Job (Package) finishes executing Run().
	JobFinished
)

// JobStatus indicates where in the execution process a Job (Package) is.
type JobStatus int

const (
	// Queued means the Job is added to the Worker but hasn't started yet.
	Queued JobStatus = 1 << iota

	// Running means the Job has started executing its Run() method.
	Running

	// Finished means the Job has finished executing its Run() method and presumably has no errors.
	Finished

	// Errored means the Job errored at some point during the Run() method and has stopped executing.
	Errored
)

type registerEntry struct {
	ch   chan bool
	lock *sync.RWMutex
}

func (lc *registerEntry) init() {
	if lc.lock == nil {
		lc.lock = new(sync.RWMutex)
	}
}

func (lc *registerEntry) Ch() chan bool {
	lc.init()

	lc.lock.RLock()
	defer lc.lock.RUnlock()

	v := lc.ch

	return v
}

func (lc *registerEntry) SetCh(ch chan bool) {
	lc.init()

	lc.lock.Lock()
	defer lc.lock.Unlock()

	lc.ch = ch
}

type register []registerEntry

func (r *register) Empty() bool {
	for i := 0; i < len(*r); i++ {
		if (*r)[i].Ch() != nil {
			return false
		}
	}

	return true
}

// Stats contains some information about the Packages that are Queued, Running, Finished, etc.
type Stats struct {
	Total    int64
	Running  int64
	Finished int64
	Queued   int64
	Errored  int64
}
