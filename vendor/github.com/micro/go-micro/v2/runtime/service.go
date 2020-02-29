package runtime

import (
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/micro/go-micro/v2/runtime/local/build"
	"github.com/micro/go-micro/v2/runtime/local/process"
	proc "github.com/micro/go-micro/v2/runtime/local/process/os"
	"github.com/micro/go-micro/v2/util/log"
)

type service struct {
	sync.RWMutex

	running bool
	closed  chan bool
	err     error
	updated time.Time

	retries    int
	maxRetries int

	// output for logs
	output io.Writer

	// service to manage
	*Service
	// process creator
	Process *proc.Process
	// Exec
	Exec *process.Executable
	// process pid
	PID *process.PID
}

func newService(s *Service, c CreateOptions) *service {
	var exec string
	var args []string

	// set command
	exec = c.Command[0]
	// set args
	if len(c.Command) > 1 {
		args = c.Command[1:]
	}

	return &service{
		Service: s,
		Process: new(proc.Process),
		Exec: &process.Executable{
			Package: &build.Package{
				Name: s.Name,
				Path: exec,
			},
			Env:  c.Env,
			Args: args,
		},
		closed:     make(chan bool),
		output:     c.Output,
		updated:    time.Now(),
		maxRetries: c.Retries,
	}
}

func (s *service) streamOutput() {
	go io.Copy(s.output, s.PID.Output)
	go io.Copy(s.output, s.PID.Error)
}

func (s *service) shouldStart() bool {
	if s.running {
		return false
	}
	return s.maxRetries <= s.retries
}

func (s *service) ShouldStart() bool {
	s.RLock()
	defer s.RUnlock()
	return s.shouldStart()
}

func (s *service) Running() bool {
	s.RLock()
	defer s.RUnlock()
	return s.running
}

// Start stars the service
func (s *service) Start() error {
	s.Lock()
	defer s.Unlock()

	if !s.shouldStart() {
		return nil
	}

	// reset
	s.err = nil
	s.closed = make(chan bool)

	if s.Metadata == nil {
		s.Metadata = make(map[string]string)
	}

	s.Metadata["status"] = "starting"
	// delete any existing error
	delete(s.Metadata, "error")

	// TODO: pull source & build binary
	log.Debugf("Runtime service %s forking new process", s.Service.Name)
	p, err := s.Process.Fork(s.Exec)
	if err != nil {
		s.Metadata["status"] = "error"
		s.Metadata["error"] = err.Error()
		return err
	}

	// set the pid
	s.PID = p
	// set to running
	s.running = true
	// set status
	s.Metadata["status"] = "running"

	if s.output != nil {
		s.streamOutput()
	}

	// wait and watch
	go s.Wait()

	return nil
}

// Stop stops the service
func (s *service) Stop() error {
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.closed:
		return nil
	default:
		close(s.closed)
		s.running = false
		if s.PID == nil {
			return nil
		}

		// set status
		s.Metadata["status"] = "stopping"

		// kill the process
		err := s.Process.Kill(s.PID)
		// wait for it to exit
		s.Process.Wait(s.PID)

		// set status
		s.Metadata["status"] = "stopped"

		// return the kill error
		return err
	}
}

// Error returns the last error service has returned
func (s *service) Error() error {
	s.RLock()
	defer s.RUnlock()
	return s.err
}

// Wait waits for the service to finish running
func (s *service) Wait() {
	// wait for process to exit
	err := s.Process.Wait(s.PID)

	s.Lock()
	defer s.Unlock()

	// save the error
	if err != nil {
		s.retries++
		s.Metadata["status"] = "error"
		s.Metadata["error"] = err.Error()
		s.Metadata["retries"] = strconv.Itoa(s.retries)

		s.err = err
	} else {
		s.Metadata["status"] = "done"
	}

	// no longer running
	s.running = false
}
