package worker

import (
	"sync"
)

// Package is a type that wraps a more generic Job object and contains some meta information about the Job used by the Worker.
type Package struct {
	ID     int64
	status JobStatus
	job    Job

	statusLock *sync.RWMutex
}

// NewPackage wraps the given Job and assigned ID in a *Package and returns it.
func NewPackage(id int64, j Job) *Package {
	p := &Package{
		ID:         id,
		status:     Queued,
		job:        j,
		statusLock: new(sync.RWMutex),
	}

	return p
}

// Job returns the Job associated with the Package.
func (p *Package) Job() Job {
	return p.job
}

// SetStatus sets the completion status of the Package.
func (p *Package) SetStatus(inc JobStatus) {
	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	p.status = inc
}

// Status returns the completion status of the package.
func (p *Package) Status() JobStatus {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()

	return p.status
}
