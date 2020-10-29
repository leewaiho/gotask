package gotask

import (
	"context"
	"log"
	"sync"
	"time"
)

func NewTaskController() *TaskController {
	ctx, cancelFn := context.WithCancel(context.Background())
	return &TaskController{
		ctx:          ctx,
		cancelFn:     cancelFn,
		mu:           sync.Mutex{},
		taskGroupMap: make(map[string]map[string]Task),
	}
}

type TaskController struct {
	ctx      context.Context
	cancelFn context.CancelFunc

	mu           sync.Mutex
	taskTimeout  time.Duration
	taskGroupMap map[string]map[string]Task
}

func (c *TaskController) Submit(task Task) error {
	return c.SubmitWithTimeout(task, c.taskTimeout)
}

func (c *TaskController) SubmitWithTimeout(task Task, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	childCtx, _ := context.WithTimeout(c.ctx, timeout)
	if err := task.Start(childCtx); err != nil {
		return err
	}

	taskReference, ok := task.(TaskReference)
	if !ok {
		log.Printf("task unimplements TaskReference so that cannot be traced by TaskController")
		return nil
	}

	groupID := taskReference.GroupID()
	taskMap := c.taskGroupMap[groupID]
	if taskMap == nil {
		taskMap = make(map[string]Task)
		c.taskGroupMap[groupID] = taskMap
	}
	taskMap[taskReference.TaskID()] = task
	return nil
}

func (c *TaskController) Cancel() {
	if c.cancelFn != nil {
		c.cancelFn()
	}
}

func (c *TaskController) CancelGroup(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	taskMap := c.taskGroupMap[groupID]
	if taskMap == nil {
		return
	}
	for k, v := range taskMap {
		if cancellable, ok := v.(Cancellable); ok {
			cancellable.Cancel()
		}
		delete(c.taskGroupMap[groupID], k)
	}
}

func (c *TaskController) CancelTask(ref TaskReference) {
	c.mu.Lock()
	defer c.mu.Unlock()

	groupID := ref.GroupID()
	taskMap := c.taskGroupMap[groupID]
	if taskMap == nil {
		return
	}

	taskID := ref.TaskID()
	task, exists := taskMap[taskID]
	if !exists {
		return
	}
	if cancellable, ok := task.(Cancellable); ok {
		cancellable.Cancel()
	}
	delete(c.taskGroupMap[groupID], taskID)
}
