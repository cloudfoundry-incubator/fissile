# worker

[ ![travis-ci status for jimmysawczuk/worker](https://travis-ci.org/jimmysawczuk/worker.svg)](https://travis-ci.org/jimmysawczuk/worker) [![GoDoc](https://godoc.org/github.com/jimmysawczuk/worker?status.svg)](https://godoc.org/github.com/jimmysawczuk/worker) [![Go Report Card](https://goreportcard.com/badge/github.com/jimmysawczuk/worker)](https://goreportcard.com/report/github.com/jimmysawczuk/worker)

Package `worker` is a Go package designed to facilitate the easy parallelization of a number of tasks `N` with up to `n` at a time being computed concurrently.

## Getting started

```bash
$ go get github.com/jimmysawczuk/worker
```

## Using in your program

### Design

To use this package, all you need to do is package your tasks into types that satisfy the following interface:

```go
type Job interface {
	Run()
}
```

### Implementation

From there, it's easy to add your task to the queue and start it:

```go
type SampleJob struct {
	Name     string
	Duration time.Duration
}

func (s *SampleJob) Run() {

	time.Sleep(s.Duration)
	log.Printf("Done, slept for %s\n", s.Duration)

}

// only do 3 jobs at a time
worker.MaxJobs = 3

w := worker.NewWorker()
w.Add(SampleJob{
	Name: "sleep 1",
	Duration: 1 * time.Second,
})

w.Add(SampleJob{
	Name: "sleep 2",
	Duration: 2 * time.Second,
})

// ... and so forth

w.RunUntilDone()
```

Your `Job`s are packaged internally as `Package`s, which have nice features such as storing a unique-per-worker ID, as well as the return value that is retrieved from the channel. This is mostly used for event handling though; keep in mind that you can store your information in this value or you can simply use your custom `Job` type and store more custom information.

### Events

You can also listen for events from the `Worker` and react appropriately. Currently, three events are fired: `JobQueued`, `JobStarted`, and `JobFinished`. Add an event handler like so:

```go
w.On(worker.JobStarted, func(pk *worker.Package, args ...interface{}) {
	// You can use type assertion to get back your original job from this:
	job := pk.Job()
})
```

Currently each event emitter only passes one argument, the relevant `Package` that emitted the event. There may be more added later, for other events, but the `Package` will always be the first argument.

## More documentation

You can find more documentation at [GoDoc][godoc].

## Examples

* [`less-tree`][less-tree], a recursive, per-directory LESS compiler uses `worker`

  [godoc]: http://godoc.org/github.com/jimmysawczuk/worker
  [less-tree]: http://github.com/jimmysawczuk/less-tree
