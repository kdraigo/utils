package sync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Future is an interface representing a combination of a future/promise and a task controller.
type Future[DataType any] interface {
	// Get blocks the current goroutine until data is received from the channel.
	// If an OnData callback is bound, calling Get will panic.
	Get() DataType

	// TryGet attempts to receive data within the specified duration.
	// If data is not ready, it returns the default DataType and false.
	// If an OnData callback is bound, calling TryGet will panic.
	TryGet(duration time.Duration) (DataType, bool)

	// OnData binds a callback to handle the received data.
	// The callback signature must be func(data DataType) error.
	// If an error is returned from the handler callback, it terminates the child goroutine.
	OnData(onData func(data DataType) error) Future[DataType]

	// OnFailure binds a callback for error handling.
	// Any error occurring during the execution of the child goroutine must be handled after termination.
	OnFailure(onFailure func(err error)) Future[DataType]

	// Cancel terminates the context associated with the child goroutine.
	Cancel()

	// Wait blocks the parent goroutine, waiting for the child to finish.
	// It returns an error if the child goroutine encountered an error.
	Wait() error

	// IsInProgress checks if the child goroutine is still in progress.
	IsInProgress() bool
}

// Promise is an interface representing a sender and notifier about events.
type Promise[DataType any] interface {
	// Send sends data through the channel.
	// If an OnDone callback is bound, invoking the handler in the current goroutine.
	Send(data DataType) error

	// CloseChannel closes the channel.
	CloseChannel()

	// Fail sets an error and terminates execution.
	Fail(err error)

	// OnDone binds a callback function for handling steps on termination.
	OnDone(onClose func()) Promise[DataType]

	// Context returns the context of the Promise.
	Context() context.Context
}

type sharedState[DataType any] struct {
	ctx    context.Context
	cancel context.CancelFunc

	done    *sync.WaitGroup
	channel chan DataType

	reason          *Error
	inProgress      *atomic.Bool
	isChannelClosed *atomic.Bool

	// Event functions from Future(receiver).
	onData              atomic.Pointer[func(data DataType) error]
	onDataInit          sync.Once
	isDataHandlerExists *atomic.Bool
	// Event functions from Promise(sender).
	onDone     atomic.Pointer[func()]
	onDoneInit sync.Once

	onFailure     atomic.Pointer[func(err error)]
	onFailureInit sync.Once
}

func newSharedStateWithCancel[DataType any](ctx context.Context, channelBuffSize int) *sharedState[DataType] {
	newContext, cancel := context.WithCancel(ctx)
	return newSharedState[DataType](newContext, cancel, channelBuffSize)
}

func newSharedStateWithTimeout[DataType any](ctx context.Context, channelBuffSize int, duration time.Duration) *sharedState[DataType] {
	newContext, cancel := context.WithTimeout(ctx, duration)
	return newSharedState[DataType](newContext, cancel, channelBuffSize)
}

func newSharedState[DataType any](ctx context.Context, cancel context.CancelFunc, channelBuffSize int) *sharedState[DataType] {
	newContext, cancel := context.WithCancel(ctx)

	sd := &sharedState[DataType]{
		ctx:                 newContext,
		cancel:              cancel,
		done:                &sync.WaitGroup{},
		channel:             make(chan DataType, channelBuffSize),
		reason:              NewError(_zeroError),
		inProgress:          &atomic.Bool{},
		isChannelClosed:     &atomic.Bool{},
		isDataHandlerExists: &atomic.Bool{},
	}

	sd.done.Add(1)
	sd.inProgress.Store(true)
	sd.isChannelClosed.Store(false)
	sd.isDataHandlerExists.Store(false)
	context.AfterFunc(sd.ctx, func() {
		if onDone := sd.onDone.Load(); onDone != nil {
			(*onDone)()
		}
		if onFailure := sd.onFailure.Load(); onFailure != nil && sd.reason.Load() != nil {
			(*onFailure)(sd.reason.Load())
		}
		sd.inProgress.Store(false)
		sd.done.Done()
	})

	return sd
}

func (sd *sharedState[DataType]) OnData(onData func(data DataType) error) Future[DataType] {
	sd.onDataInit.Do(func() {
		sd.onData.Store(&onData)
		sd.isDataHandlerExists.Store(true)
	})

	return sd
}

func (sd *sharedState[DataType]) OnFailure(onFailure func(err error)) Future[DataType] {
	sd.onFailureInit.Do(func() {
		sd.onFailure.Store(&onFailure)
	})
	return sd
}

func (sd *sharedState[DataType]) OnDone(onDone func()) Promise[DataType] {
	sd.onDoneInit.Do(func() {
		sd.onDone.Store(&onDone)
	})

	return sd
}

func (sd *sharedState[DataType]) Get() DataType {
	if sd.isDataHandlerExists.Load() {
		panic("callback already bound")
	}
	return <-sd.channel
}

func (sd *sharedState[DataType]) TryGet(duration time.Duration) (DataType, bool) {
	var data DataType
	ok := false
	if sd.isDataHandlerExists.Load() {
		panic("callback already bound")
	}
	select {
	case data, ok = <-sd.channel:
		return data, ok
	case <-time.After(duration):
		return data, ok
	}
}

func (sd *sharedState[DataType]) Cancel() {
	if sd.cancel != nil {
		sd.cancel()
	}
}

func (sd *sharedState[DataType]) Wait() error {
	sd.done.Wait()
	return sd.reason.Load()
}

func (sd *sharedState[DataType]) IsInProgress() bool {
	return sd.inProgress.Load()
}

func (sd *sharedState[DataType]) Context() context.Context {
	return sd.ctx
}

func (sd *sharedState[DataType]) Send(data DataType) error {
	if sd.isChannelClosed.Load() {
		return fmt.Errorf("channel closed")
	}
	if sd.onData.Load() != nil {
		if err := (*sd.onData.Load())(data); err != nil {
			sd.reason.Store(err)
			sd.Cancel()
		}
	} else {
		sd.channel <- data
	}

	return nil
}

func (sd *sharedState[DataType]) CloseChannel() {
	if sd.isChannelClosed.CompareAndSwap(false, true) {
		close(sd.channel)
	}
}

func (sd *sharedState[DataType]) Fail(err error) {
	if err == nil {
		return
	}
	sd.reason.Store(err)
	sd.Cancel()
}

// AsyncTask runs a task in a separate goroutine with handlers.
// It takes a context, channel buffer size, and an asynchronous function.
// The asynchronous function receives a Promise, which is used for sending data,
// handling termination steps, and managing the context.
func AsyncTask[DataType any](
	ctx context.Context,
	channelBufSize int,
	asyncFunc func(promise Promise[DataType]),
) Future[DataType] {
	// Create a shared state with cancellation for the asynchronous task.
	sharedState := newSharedStateWithCancel[DataType](ctx, channelBufSize)

	// Start a new goroutine to execute the asynchronous function.
	go func() {
		// Ensure cancellation of the shared state on goroutine exit.
		defer sharedState.Cancel()

		// Execute the asynchronous function, passing the shared state.
		asyncFunc(sharedState)
	}()

	// Return the shared state as a Future for monitoring progress.
	return sharedState
}

// AsyncTaskWithTimeOut runs a task in a separate goroutine with handlers and a timeout.
// It takes a context, channel buffer size, timeout duration, and an asynchronous function.
// The asynchronous function receives a Promise, which is used for sending data,
// handling termination steps, and managing the context.
// The task will be canceled if it exceeds the specified timeout duration.
func AsyncTaskWithTimeOut[DataType any](
	ctx context.Context,
	channelBufSize int,
	duration time.Duration,
	asyncFunc func(promise Promise[DataType]),
) Future[DataType] {
	// Create a shared state with timeout for the asynchronous task.
	sharedState := newSharedStateWithTimeout[DataType](ctx, channelBufSize, duration)

	// Start a new goroutine to execute the asynchronous function.
	go func() {
		// Ensure cancellation of the shared state on goroutine exit.
		defer sharedState.Cancel()

		// Execute the asynchronous function, passing the shared state.
		asyncFunc(sharedState)
	}()

	// Return the shared state as a Future for monitoring progress.
	return sharedState
}
