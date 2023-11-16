package sync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Future is receiver and task controller.
type Future[DataType any] interface {
	Get() DataType
	TryGet(duration time.Duration) (DataType, bool)

	OnData(onData func(data DataType) error) Future[DataType]
	OnFailure(onFailure func(err error)) Future[DataType]

	Cancel()
	Wait() error
	IsInProgress() bool
}

// Promise is sender and notifier about events.
type Promise[DataType any] interface {
	Send(data DataType) error
	CloseChannel()
	Fail(err error)
	OnDone(onClose func()) Promise[DataType]
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
		var data DataType
		return data
	}
	return <-sd.channel
}

func (sd *sharedState[DataType]) TryGet(duration time.Duration) (DataType, bool) {
	var data DataType
	ok := false
	if sd.isDataHandlerExists.Load() {
		return data, ok
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

// AsyncTask runs separate task in goroutine with handlers.
func AsyncTask[DataType any](ctx context.Context, channelBufSize int,
	asyncFunc func(promise Promise[DataType])) Future[DataType] {

	sharedState := newSharedStateWithCancel[DataType](ctx, channelBufSize)
	go func() {
		defer sharedState.Cancel()
		asyncFunc(sharedState)
	}()
	return sharedState
}

// AsyncTaskWithTimeOut runs separate task in goroutine with handlers and timeout.
func AsyncTaskWithTimeOut[DataType any](ctx context.Context, channelBufSize int, duration time.Duration,
	asyncFunc func(promise Promise[DataType])) Future[DataType] {

	sharedState := newSharedStateWithTimeout[DataType](ctx, channelBufSize, duration)
	go func() {
		defer sharedState.Cancel()
		asyncFunc(sharedState)
	}()
	return sharedState
}
