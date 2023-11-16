package sync

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type SharedStateSuite struct {
	suite.Suite
}

func TestSharedStateSuite(t *testing.T) {
	suite.Run(t, new(SharedStateSuite))
}

func (ss *SharedStateSuite) SetupTest() {
	// Setup any common test fixtures here
}

func (ss *SharedStateSuite) TestAsyncTask() {
	// Your test logic for AsyncTask
}

func (ss *SharedStateSuite) TearDownTest() {
}

func (ss *SharedStateSuite) TestSendAndReceiveWithCancel() {
	ctx := context.Background()
	channelBufSize := 4

	i := 0
	asyncFunc := func(promise Promise[int]) {
		for ; i < channelBufSize+1; i++ {
			promise.Send(i)
		}
		promise.CloseChannel()
		ss.Equal(fmt.Errorf("channel closed"), promise.Send(8))
	}

	future := AsyncTask[int](ctx, channelBufSize, asyncFunc)
	j := 0
	for ; j < channelBufSize+1; j++ {
		ss.Equal(j, future.Get())
	}
	ss.Nil(future.Wait())
	ss.Equal(j, i)

	asyncFunc = func(promise Promise[int]) {
		for ; i < channelBufSize+1; i++ {
			promise.Send(i)
		}
	}
	future = AsyncTask[int](ctx, channelBufSize, asyncFunc)
	j = 0
	for ; j < channelBufSize+1; j++ {
		data, isReceived := future.TryGet(time.Duration(1 * time.Second))
		if isReceived {
			ss.Equal(j, data)
		}
	}
	_, isReceived := future.TryGet(time.Duration(1 * time.Second))
	ss.False(isReceived)
	ss.Nil(future.Wait())
	ss.Equal(j, i)
}

func (ss *SharedStateSuite) TestHandlersWithCancel() {
	ctx := context.Background()
	channelBufSize := 0
	iter := atomic.Int32{}

	expectedError := fmt.Errorf("invalid data")
	future := AsyncTask[int32](ctx, channelBufSize,
		func(promise Promise[int32]) {
			for iter.Load() < 5 {
				promise.OnDone(func() {
					ss.True(iter.CompareAndSwap(iter.Load(), iter.Load()+1))
				})
				promise.Send(iter.Load())
				ss.True(iter.CompareAndSwap(iter.Load(), iter.Load()+1))
			}
		}).
		OnData(
			func(data int32) error {
				if data == 4 {
					return fmt.Errorf("invalid data")
				}
				ss.Equal(iter.Load(), data)
				return nil
			}).
		OnFailure(func(err error) {
			ss.True(iter.CompareAndSwap(iter.Load(), iter.Load()+1))
			ss.Equal(expectedError, err)
		})
	ss.Equal(expectedError, future.Wait())
	ss.Equal(int32(7), iter.Load())

	future = AsyncTask[int32](ctx, channelBufSize,
		func(promise Promise[int32]) {
			promise.OnDone(func() {
				ss.True(iter.CompareAndSwap(iter.Load(), iter.Load()+1))
			})

			for iter.Load() < 10 {
				promise.Send(iter.Load())
				ss.True(iter.CompareAndSwap(iter.Load(), iter.Load()+1))
			}
		}).
		OnData(func(data int32) error {
			ss.Equal(iter.Load(), data)
			return nil
		})

	ss.Nil(future.Wait())
	ss.Equal(int32(11), iter.Load())
}

func (ss *SharedStateSuite) TestHandlersWithCancelChain() {
	ctx := context.Background()
	channelBufSize := 0
	iter := atomic.Int32{}
	someErrorFromFirstPromise := fmt.Errorf("some error from first promise")
	someErrorFromSecondPromise := fmt.Errorf("some error from second promise")

	firstFuture := AsyncTask[int32](ctx, channelBufSize,
		func(firstPromise Promise[int32]) {
			firstPromise.OnDone(func() {
				iter.Add(1)
			})

			AsyncTask[int32](firstPromise.Context(), channelBufSize,
				func(secondPromise Promise[int32]) {
					secondPromise.OnDone(func() {
						iter.Add(1)
					})

					// Some job here with thirdPromise
					thirdFuture := AsyncTask[string](secondPromise.Context(), channelBufSize,
						func(thirdPromise Promise[string]) {
							thirdPromise.OnDone(func() {
								iter.Add(1)
							})
							iter.Add(1)
							thirdPromise.Send("some string")
						}).
						OnData(func(data string) error {
							ss.Equal("some string", data)
							iter.Add(1)
							return nil
						})
					// Task complete without any errors.
					ss.Nil(thirdFuture.Wait())

					iter.Add(1)
					secondPromise.Send(iter.Load())
				}).
				OnData(func(data int32) error {
					ss.Equal(iter.Load(), data)
					// Something happened in data handling.
					return someErrorFromSecondPromise
				}).
				OnFailure(func(err error) {
					iter.Add(1)
					ss.Equal(someErrorFromSecondPromise, err)
					// Broadcasting error to parent promise.
					firstPromise.Fail(fmt.Errorf("%v %v", someErrorFromFirstPromise, err))
				}).Wait()

			// Unreachable. firstPromise OnFailure callback will invoke before this increment.
			firstPromise.Send(iter.Load())

		}).
		OnData(func(data int32) error {
			iter.Add(1)
			return nil
		}).
		OnFailure(func(err error) {
			iter.Add(1)
		})

	ss.Equal(fmt.Errorf("%v %v", someErrorFromFirstPromise, someErrorFromSecondPromise), firstFuture.Wait())
	ss.Equal(int32(9), iter.Load())

}

func (ss *SharedStateSuite) TestHandlersWithTimeout() {
	ctx := context.Background()
	channelBufSize := 0
	cancelAfter := time.Duration(3 * time.Second)

	firstIter := atomic.Int32{}
	firstIter.Store(1)
	future := AsyncTaskWithTimeOut[int32](ctx, channelBufSize, cancelAfter,
		func(promise Promise[int32]) {
			for ; firstIter.Load() < 10; firstIter.Add(1) {
				// Sending just one value 0.
				promise.Send(firstIter.Load())
				// Must canceled during sleeping.
				time.Sleep(10 * time.Second)
			}
		}).
		OnData(func(data int32) error {
			return nil
		})
	ss.Nil(future.Wait())
	ss.Equal(int32(1), firstIter.Load())

	secondIter := atomic.Int32{}
	expectedError := fmt.Errorf("somme error from task")
	future = AsyncTaskWithTimeOut[int32](ctx, channelBufSize, cancelAfter,
		func(promise Promise[int32]) {
			promise.OnDone(func() {
				secondIter.Add(1)
			})
			for ; secondIter.Load() < 10; secondIter.Add(1) {
				promise.Send(int32(secondIter.Load()))
			}
		}).
		OnData(func(data int32) error {
			ss.Equal(secondIter.Load(), data)
			if secondIter.Load() == int32(9) {
				return expectedError
			}
			return nil
		}).
		OnFailure(func(err error) {
			secondIter.Add(1)
			ss.Equal(expectedError, err)
		})

	ss.Equal(expectedError, future.Wait())
	ss.Equal(int32(12), secondIter.Load())

	c := atomic.Int32{}
	f := atomic.Int32{}
	futureStr := AsyncTaskWithTimeOut[string](ctx, channelBufSize, cancelAfter,
		func(promise Promise[string]) {
			promise.OnDone(func() {
				c.Store(333)
				promise.Fail(expectedError)
			})
			for {
				// Infinite loop.
				promise.Send("int32()")
			}
		}).
		OnData(func(data string) error {
			return nil
		}).
		OnFailure(func(err error) {
			f.Store(112)
		})

	ss.Equal(expectedError, futureStr.Wait())
	ss.Equal(int32(112), f.Load())
	ss.Equal(int32(333), c.Load())
}
