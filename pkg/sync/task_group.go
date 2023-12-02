package sync

import (
	"context"
	"sync"
	"time"
)

// Task presenting subtask.
type Task interface {
	Cancel()
	Wait() error
	IsInProgress() bool
}

// TaskGroup for tracking and managing subtasks.
type TaskGroup struct {
	signalHandlerOnce sync.Once
	ctx               context.Context
	tasksGuard        *sync.RWMutex
	tasks             []Task
	statistics        []error
}

// NewTaskGroup crating Tasks instance.
func NewTaskGroup(ctx context.Context) *TaskGroup {
	return &TaskGroup{
		ctx:        ctx,
		tasksGuard: &sync.RWMutex{},
	}
}

// BindTask binding task.
func (t *TaskGroup) BindTask(task Task) {
	t.tasksGuard.Lock()
	defer t.tasksGuard.Unlock()
	t.tasks = append(t.tasks, task)
}

// BindSignalHandler binding signal handler function.
func (t *TaskGroup) BindSignalHandler(sigHandler func()) {
	t.signalHandlerOnce.Do(func() {
		go func() {
			defer t.shutDownAll()
			sigHandler()
		}()
	})
}

// WaitForTasks tracking all subtasks by given predicate.
func (t *TaskGroup) WaitForTasks(shutDownOn func(t Task) bool, checkPer time.Duration) {
	defer t.shutDownAll()
	needShutDown := false
	for {
		t.tasksGuard.Lock()
		isNotInProgress := 0
		for _, ct := range t.tasks {
			if !ct.IsInProgress() {
				isNotInProgress++
			}
			if needShutDown = shutDownOn(ct); needShutDown {
				break
			}
		}
		t.tasksGuard.Unlock()
		if isNotInProgress == len(t.tasks) {
			break
		}
		if needShutDown {
			break
		}
		time.Sleep(checkPer)
	}
}

// Statistics returning subtasks execution errors.
func (t *TaskGroup) Statistics() []error {
	t.tasksGuard.Lock()
	defer t.tasksGuard.Unlock()

	return t.statistics
}

// shutDownAll shutting down all subtasks and combining statistics.
func (t *TaskGroup) shutDownAll() {
	t.tasksGuard.Lock()
	defer t.tasksGuard.Unlock()

	for _, ct := range t.tasks {
		ct.Cancel()
	}
	for _, ct := range t.tasks {
		if err := ct.Wait(); err != nil {
			t.statistics = append(t.statistics, err)
		}
	}
}
