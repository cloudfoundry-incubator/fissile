// Package sigint provides a Handler that calls exit handlers when the process
// has been interrupted by sigint (a ctrl-c typically).
package sigint

import (
	"os"
	"os/signal"
	"sync"
)

const (
	// SigInt is the exit code that the shell returns if terminated with CTRL-C
	SigInt = 130
)

var (
	// DefaultHandler is the singleton for this package, because of the nature
	// of os.Exit() there's really no point to have multiple Handler's lying
	// around as only one can ever be executed.
	DefaultHandler *Handler
)

func init() {
	DefaultHandler = NewHandler()
	go DefaultHandler.Run()
}

// Handler waits on a sigint and exits the program after calling the set of
// callbacks.
type Handler struct {
	sigintChan chan os.Signal

	sync.Mutex
	callbacks []func()
}

// NewHandler creates a new sigint handler. This starts a goroutine
// automatically to handle the signal.
func NewHandler() *Handler {
	sigintChan := make(chan os.Signal)
	signal.Notify(sigintChan, os.Interrupt)

	handler := &Handler{
		sigintChan: sigintChan,
	}

	return handler
}

// Add callbacks to the list of sigint handlers to call when the signal is
// received.
func (h *Handler) Add(callbacks ...func()) {
	h.Lock()
	defer h.Unlock()

	h.callbacks = append(h.callbacks, callbacks...)
}

// test harness
var (
	exitFunction = os.Exit
)

// Exit the process with an exit code.
func (h *Handler) Exit(code int) {
	h.Lock()
	defer h.Unlock()

	for _, callback := range h.callbacks {
		callback()
	}

	exitFunction(code)
}

// Run the handler, waiting for the signal.
func (h *Handler) Run() {
	for {
		select {
		case signal := <-h.sigintChan:
			if signal == os.Interrupt {
				h.Exit(SigInt)
			}
		}
	}
}
