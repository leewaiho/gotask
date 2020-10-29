package gotask

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"

	"github.com/pkg/errors"
)

type ExecTask struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	Stdout     io.WriteCloser
	Stderr     io.WriteCloser
	closeHooks []func()
	startOnce  sync.Once
	finishOnce sync.Once
	doneCh     chan struct{}
	doneErr    error

	taskID   string
	groupID  string
	execName string
	execArgv []string
}

func (t *ExecTask) AddCloseHook(fn func()) {
	t.closeHooks = append(t.closeHooks, fn)
}

func (t *ExecTask) Info(format string, argv ...interface{}) {
	stdout := t.Stdout
	if stdout == nil {
		return
	}
	_, _ = stdout.Write([]byte(fmt.Sprintf(format, argv...)))
}

func (t *ExecTask) Error(format string, argv ...interface{}) {
	stderr := t.Stderr
	if stderr == nil {
		return
	}
	_, _ = stderr.Write([]byte(fmt.Sprintf(format, argv...)))
}

func (t *ExecTask) TaskID() string {
	return t.taskID
}

func (t *ExecTask) GroupID() string {
	return t.groupID
}

func (t *ExecTask) Start(ctx context.Context) error {
	var err error
	t.startOnce.Do(func() {
		t.ctx, t.cancelFunc = context.WithCancel(ctx)
		cmd := exec.CommandContext(t.ctx, t.execName, t.execArgv...)
		if serr := cmd.Start(); serr != nil {
			err = errors.Wrapf(serr, "start command(name: %s, argv: %v)", t.execName, t.execArgv)
			return
		}
		t.doneCh = make(chan struct{})
		go func() {
			processErr := cmd.Wait()
			t.finishOnce.Do(func() {
				t.doneErr = processErr
				close(t.doneCh)
			})
		}()
	})
	return err
}

func (t *ExecTask) Wait() error {
	if t.doneCh == nil {
		return errors.New("task is not started")
	}
	<-t.doneCh
	return t.doneErr
}

func (t *ExecTask) Cancel() {
	t.finishOnce.Do(func() {
		if t.cancelFunc != nil {
			t.cancelFunc()
			close(t.doneCh)
		}
	})
}

func (t *ExecTask) Close() error {
	for _, hookFunc := range t.closeHooks {
		if hookFunc != nil {
			hookFunc()
		}
	}
	if stdout := t.Stdout; stdout != nil {
		if err := stdout.Close(); err != nil {
			log.Printf("%+v", errors.Wrap(err, "close task resource (stdout)"))
		}
	}
	if stderr := t.Stderr; stderr != nil {
		if err := stderr.Close(); err != nil {
			log.Printf("%+v", errors.Wrap(err, "close task resource (stderr)"))
		}
	}
	t.Cancel()
	return nil
}
