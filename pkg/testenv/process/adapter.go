// Package process runs a dedicated application process for each test environment.
package process

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
)

type Resources interface {
	Prepare(context.Context, string) ([]string, error)
	Dispose(context.Context, string) error
}

type Config struct {
	Command      []string
	Directory    string
	Environment  []string
	Artifacts    string
	Revision     string
	Capabilities []string
	Resources    Resources
	ReadyPath    string
	Probe        func(context.Context, testenv.Descriptor) error
	Configure    func(testenv.Descriptor) []string
	// Initialize prepares descriptor-dependent state before application workers start.
	Initialize func(context.Context, testenv.Descriptor) error
	Manifest   testdb.Manifest
}

type child struct {
	mu       sync.Mutex
	command  *exec.Cmd
	done     chan struct{}
	exitErr  error
	disposed bool
	log      *os.File
}

type Adapter struct {
	config   Config
	mu       sync.Mutex
	children map[string]*child
}

func New(config Config) (*Adapter, error) {
	if len(config.Command) == 0 || config.Artifacts == "" || config.Revision == "" || config.Resources == nil {
		return nil, fmt.Errorf("process: command, artifacts, revision and resources are required")
	}
	return &Adapter{config: config, children: map[string]*child{}}, nil
}

func (a *Adapter) Start(ctx context.Context, spec testenv.Spec, id string) (testenv.Descriptor, error) {
	d := testenv.Descriptor{EnvironmentID: id, BuildRevision: a.config.Revision, SchemaFingerprint: a.config.Manifest.SchemaFingerprint, BaselineFingerprint: a.config.Manifest.BaselineFingerprint, Capabilities: a.config.Capabilities, ArtifactDirectory: filepath.Join(a.config.Artifacts, id)}
	if _, err := uuid.Parse(id); err != nil {
		return d, fmt.Errorf("process: invalid environment ID")
	}
	a.mu.Lock()
	if _, exists := a.children[id]; exists {
		a.mu.Unlock()
		return d, fmt.Errorf("process: environment already owned")
	}
	c := &child{}
	c.mu.Lock()
	a.children[id] = c
	a.mu.Unlock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(d.ArtifactDirectory, 0700); err != nil {
		return d, err
	}
	env, err := a.config.Resources.Prepare(ctx, id)
	if err != nil {
		return d, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return d, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return d, err
	}
	d.BaseURL = "http://127.0.0.1:" + strconv.Itoa(port)
	if a.config.Initialize != nil {
		if err := a.config.Initialize(ctx, d); err != nil {
			return d, err
		}
	}
	log, err := os.OpenFile(filepath.Join(d.ArtifactDirectory, "server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return d, err
	}
	command := exec.Command(a.config.Command[0], a.config.Command[1:]...)
	ownProcessGroup(command)
	command.Dir = a.config.Directory
	command.Env = append(append(append(os.Environ(), a.config.Environment...), env...), "HTTP_PORT="+strconv.Itoa(port), "HTTP_DOMAIN=127.0.0.1", "HTTP_ORIGIN="+d.BaseURL)
	if a.config.Configure != nil {
		command.Env = append(command.Env, a.config.Configure(d)...)
	}
	command.Stdout, command.Stderr = log, log
	c.command, c.done, c.log = command, make(chan struct{}), log
	if err := command.Start(); err != nil {
		c.command = nil
		return d, err
	}
	go func() { c.exitErr = command.Wait(); close(c.done) }()
	return d, nil
}

func (a *Adapter) Ready(ctx context.Context, d testenv.Descriptor) error {
	a.mu.Lock()
	c := a.children[d.EnvironmentID]
	a.mu.Unlock()
	if c == nil {
		return fmt.Errorf("process: environment not started")
	}
	c.mu.Lock()
	started, done := c.command != nil, c.done
	c.mu.Unlock()
	if !started {
		return fmt.Errorf("process: environment not started")
	}
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.BaseURL+a.config.ReadyPath, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 400 {
				if a.config.Probe == nil {
					return nil
				}
				if err := a.config.Probe(ctx, d); err == nil {
					return nil
				}
			}
		}
		select {
		case <-done:
			return fmt.Errorf("process exited before readiness: %v", c.exitErr)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (a *Adapter) Stop(ctx context.Context, d testenv.Descriptor) error {
	a.mu.Lock()
	c := a.children[d.EnvironmentID]
	a.mu.Unlock()
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disposed {
		return nil
	}
	if c.command != nil {
		_ = interruptOwned(c.command)
		select {
		case <-c.done:
		case <-ctx.Done():
			_ = killOwned(c.command)
			<-c.done
		case <-time.After(2 * time.Second):
			_ = killOwned(c.command)
			<-c.done
		}
		c.command = nil
	}
	if c.log != nil {
		_ = c.log.Close()
		c.log = nil
	}
	if err := a.config.Resources.Dispose(ctx, d.EnvironmentID); err != nil {
		return errors.Join(fmt.Errorf("process: dispose resources"), err)
	}
	c.disposed = true
	a.mu.Lock()
	if a.children[d.EnvironmentID] == c {
		delete(a.children, d.EnvironmentID)
	}
	a.mu.Unlock()
	return nil
}
