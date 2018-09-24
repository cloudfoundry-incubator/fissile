// Package worker accepts Jobs and places them in a queue to be executed N at a time.
package worker

import (
	"sync"
	"time"
)

// MaxJobs is the default amount of jobs to run at a time. This can be changed per Worker object as well.
var MaxJobs = 4

// A Job is an object with a Run() method, which is expected to complete a given task. Jobs can be run in parallel, so any resources shared
// between Jobs should be thread-safe.
type Job interface {
	Run()
}

// A Worker holds and executes a bunch of jobs N at a time.
type Worker struct {
	maxJobs int

	events map[Event][]func(*Package, ...interface{})

	nextID int64
	idLock sync.Mutex

	started lockSwitch

	queue       Queue
	jobs        Map
	runningJobs register
}

// NewWorker returns a new Worker object with the maximum jobs at a time set to the default.
func NewWorker() *Worker {

	w := &Worker{}
	w.reset()

	w.builtInEvents()

	return w
}

func (w *Worker) reset() {
	w.nextID = 1
	w.queue = NewQueue()
	w.jobs = NewMap()

	if w.maxJobs == 0 && MaxJobs > 0 {
		w.maxJobs = MaxJobs
	} else {
		w.maxJobs = 1
	}

	w.events = make(map[Event][]func(*Package, ...interface{}))
	w.runningJobs = make(register, w.maxJobs)
}

func (w *Worker) builtInEvents() {
	w.events = make(map[Event][]func(*Package, ...interface{}))

	w.On(jobFinished, w.jobFinished)
}

func (w *Worker) getNextID() int64 {
	w.idLock.Lock()
	defer w.idLock.Unlock()

	thisID := w.nextID
	w.nextID++
	return thisID
}

// Add adds a Job to the Worker's queue
func (w *Worker) Add(j Job) {
	p := NewPackage(w.getNextID(), j)

	w.jobs.Set(p)
	w.queue.Add(p)

	w.emit(jobAdded, w.jobs.Get(p.ID))
	w.emit(JobAdded, w.jobs.Get(p.ID))
}

// On attaches an event handler to a given Event.
func (w *Worker) On(e Event, cb func(*Package, ...interface{})) {
	if _, exists := w.events[e]; !exists {
		w.events[e] = make([]func(*Package, ...interface{}), 0)
	}

	w.events[e] = append(w.events[e], cb)
}

func (w *Worker) emit(e Event, pk *Package, arguments ...interface{}) {
	if _, exists := w.events[e]; exists {
		for _, v := range w.events[e] {
			v(pk, arguments...)
		}
	}
}

// RunUntilDone tells the Worker to run until all of its jobs are completed and then shut down and stop accepting Jobs.
func (w *Worker) RunUntilDone() {
	w.runUntilDone()
}

// RunUntilStopped tells the Worker to run until it's explicitly told to stop via an ExitCode. It'll accept new Jobs until this happens.
func (w *Worker) RunUntilStopped(stopCh chan ExitCode) {
	internalCh := make(chan ExitCode)
	go w.runUntilKilled(stopCh, internalCh)
	ret := <-internalCh
	stopCh <- ret
}

func (w *Worker) runUntilKilled(killCh chan ExitCode, returnCh chan ExitCode) {
	if !w.started.On() {
		w.started.Set(true)
		defer w.started.Set(false)

		exit := false

		for {
			select {
			case code := <-killCh:
				if code == ExitWhenDone {
					exit = true
				}

			default:
				for i := 0; i < len(w.runningJobs); i++ {
					if w.runningJobs[i].Ch() == nil {
						p := w.queue.Top()
						if p == nil {
							break
						}

						w.runningJobs[i].SetCh(make(chan bool))
						go (func(i int) {
							<-w.runningJobs[i].Ch()
							w.runningJobs[i].SetCh(nil)
						})(i)
						go w.runJob(p, w.runningJobs[i].Ch())
					}
				}

				if exit && w.queue.Len() == 0 && w.runningJobs.Empty() {
					returnCh <- ExitWhenDone
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}

	returnCh <- ExitNormally
	return
}

func (w *Worker) runUntilDone() {
	if !w.started.On() {
		w.started.Set(true)
		defer w.started.Set(false)

		for w.queue.Len() > 0 || !w.runningJobs.Empty() {

			for i := 0; i < len(w.runningJobs); i++ {
				if w.runningJobs[i].Ch() == nil {
					p := w.queue.Top()
					if p == nil {
						break
					}

					w.runningJobs[i].SetCh(make(chan bool))
					go (func(i int) {
						<-w.runningJobs[i].Ch()
						w.runningJobs[i].SetCh(nil)
					})(i)
					go w.runJob(p, w.runningJobs[i].Ch())
				}
			}

			time.Sleep(5 * time.Millisecond)
		}
	} else {
		for w.queue.Len() > 0 || !w.runningJobs.Empty() {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (w *Worker) runJob(p *Package, returnCh chan bool) {

	// log.Printf("Starting job %d", p.ID)
	jobCh := make(chan bool)
	go func(jobCh chan bool) {
		// log.Printf("Running job %d", p.ID)
		p.job.Run()
		jobCh <- true
	}(jobCh)

	p.SetStatus(Running)

	w.emit(jobStarted, w.jobs.Get(p.ID))
	w.emit(JobStarted, w.jobs.Get(p.ID))

	_ = <-jobCh

	// log.Printf("Job %d finished", p.ID)
	w.emit(jobFinished, w.jobs.Get(p.ID))
	w.emit(JobFinished, w.jobs.Get(p.ID))

	returnCh <- true
}

func (w *Worker) jobFinished(pk *Package, args ...interface{}) {
	pk.SetStatus(Finished)
}

// Stats returns a collection of statistics related to how many jobs are finished, queued, running, etc.
func (w *Worker) Stats() (stats Stats) {
	for _, p := range w.jobs.jobs {
		switch p.Status() {
		case Queued:
			stats.Queued++
		case Running:
			stats.Running++
		case Finished:
			stats.Finished++
		case Errored:
			stats.Errored++
		}

		stats.Total++
	}

	return
}
