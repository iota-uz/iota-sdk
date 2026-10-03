package testenv

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

type controlRequest struct {
	ID            string `json:"id"`
	Operation     string `json:"operation"`
	Spec          Spec   `json:"spec"`
	EnvironmentID string `json:"environmentId"`
}
type response struct {
	ID         string      `json:"id"`
	Descriptor *Descriptor `json:"descriptor,omitempty"`
	Error      *Error      `json:"error,omitempty"`
}

// Serve handles the local NDJSON control protocol and stops owned resources on EOF or cancellation.
func Serve(ctx context.Context, c *Coordinator, reader io.Reader, writer io.Writer) (returnErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		returnErr = errors.Join(returnErr, c.StopAll(cleanup))
	}()
	lines := make(chan []byte)
	scanErrors := make(chan error, 1)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- b:
			case <-ctx.Done():
				scanErrors <- ctx.Err()
				return
			}
		}
		scanErrors <- scanner.Err()
	}()
	encoder := json.NewEncoder(writer)
	var writing sync.Mutex
	var pending sync.WaitGroup
	capacity := make(chan struct{}, 4)
	writeErrors := make(chan error, 1)
	defer func() {
		pending.Wait()
		select {
		case err := <-writeErrors:
			returnErr = errors.Join(returnErr, err)
		default:
		}
	}()
	respond := func(line []byte) {
		defer pending.Done()
		defer func() { <-capacity }()
		var req controlRequest
		result := response{}
		if err := json.Unmarshal(line, &req); err != nil {
			result.Error = &Error{Code: "invalid_input", Message: err.Error()}
		} else {
			result.ID = req.ID
			call, done := context.WithTimeout(ctx, 60*time.Second)
			defer done()
			var err error
			switch req.Operation {
			case "start":
				d, startErr := c.Start(call, req.Spec)
				err = startErr
				if d.EnvironmentID != "" {
					result.Descriptor = &d
				}
			case "stop":
				err = c.Stop(call, req.EnvironmentID)
			default:
				err = &Error{Code: "invalid_input", Message: "unknown operation"}
			}
			if err != nil {
				d := Descriptor{EnvironmentID: req.EnvironmentID}
				if result.Descriptor != nil {
					d = *result.Descriptor
				}
				result.Error = lifecycleError(err, "execution_failed", req.Operation, d, nil)
			}
		}
		writing.Lock()
		err := encoder.Encode(result)
		writing.Unlock()
		if err != nil {
			select {
			case writeErrors <- err:
			default:
			}
			cancel()
		}
	}
	for {
		select {
		case <-ctx.Done():
			select {
			case err := <-writeErrors:
				return err
			default:
				return ctx.Err()
			}
		case line, ok := <-lines:
			if !ok {
				return <-scanErrors
			}
			select {
			case capacity <- struct{}{}:
				pending.Add(1)
				go respond(line)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
