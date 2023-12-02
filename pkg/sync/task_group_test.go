package sync

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TaskGroupSuite struct {
	suite.Suite
}

func TestTaskGroupSuiteSuite(t *testing.T) {
	suite.Run(t, new(TaskGroupSuite))
}

func (ss *TaskGroupSuite) SetupTest() {
	// Setup any common test fixtures here
}

func (ss *TaskGroupSuite) TearDownTest() {
}

func (ss *TaskGroupSuite) TestSubtasksWaiting() {
	iter := atomic.Int32{}
	asyncFunc := func(promise Promise[int32]) {
		iter.Add(1)
	}
	onShutDownPred := func(future Task) bool {
		return false
	}

	tasksGroup := NewTaskGroup(context.Background())
	groupSize := 100
	for i := 0; i < groupSize; i++ {
		tasksGroup.BindTask(AsyncTask[int32](context.Background(), 0, asyncFunc))
	}

	tasksGroup.WaitForTasks(onShutDownPred, time.Duration(1*time.Second))
	ss.Equal(int32(groupSize), iter.Load())
}

func (ss *TaskGroupSuite) TestWaitingForError() {
	// Testing shutdown with error.
	onShutDownPred := func(future Task) bool {
		if !future.IsInProgress() {
			if err := future.Wait(); err != nil {
				// *Or we can check specified error type.
				// On this condition we want to terminate all tasks.
				return true
			}
		}
		return false
	}
	iter := atomic.Int32{}
	expectedError := fmt.Errorf("error from subtask")
	errorExitsFrom := atomic.Bool{}
	asyncFunc := func(promise Promise[int32]) {
		if iter.Load() < 51 {
			iter.Add(1)
		} else if iter.Load() == 51 && errorExitsFrom.CompareAndSwap(false, true) {
			promise.Fail(expectedError)
		}
		time.Sleep(10 * time.Second)
		// Unreachable
		iter.Add(1)
	}
	groupSize := 100
	tasksGroup := NewTaskGroup(context.Background())
	for i := 0; i < groupSize; i++ {
		tasksGroup.BindTask(AsyncTask[int32](context.Background(), 0, asyncFunc))
	}
	tasksGroup.WaitForTasks(onShutDownPred, time.Duration(1*time.Second))
	ss.Equal(int32(51), iter.Load())
	ss.Equal(1, len(tasksGroup.Statistics()))
	ss.Equal(expectedError, tasksGroup.Statistics()[0])
}

func (ss *TaskGroupSuite) TestWaitForSignal() {
	// Testing shutdown by signal.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	iter := atomic.Int32{}
	interruptedBySignal := false
	signalHandlerFunc := func() {
		// Returning on signal and shitting down all.
		<-sigChan
		interruptedBySignal = true
	}

	asyncFunc := func(promise Promise[int32]) {
		iter.Add(1)
		time.Sleep(10 * time.Second)
		// Unreachable
		iter.Add(1)
	}
	onShutDownPred := func(f Task) bool {
		return false
	}
	groupSize := 100
	tasksGroup := NewTaskGroup(context.Background())
	for i := 0; i < groupSize; i++ {
		tasksGroup.BindTask(AsyncTask[int32](context.Background(), 0, asyncFunc))
	}
	go func() {
		time.Sleep(2 * time.Second)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()
	tasksGroup.BindSignalHandler(signalHandlerFunc)
	tasksGroup.WaitForTasks(onShutDownPred, time.Duration(1*time.Second))

	ss.Equal(int32(groupSize), iter.Load())
	ss.True(interruptedBySignal)
}
