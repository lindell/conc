package conc

import (
	"context"
	"fmt"
	"sync"
)

type mapOptions struct {
	maxConcurrency int
	ctx            context.Context
}

// MapSetting is a setting for the Map function
type MapSetting func(*mapOptions)

// WithMaxConcurrency sets the maximum number of concurent go-routines
func WithMaxConcurrency(maxConcurrency int) MapSetting {
	return func(mo *mapOptions) {
		mo.maxConcurrency = maxConcurrency
	}
}

// WithContext sets the context to be used
func WithContext(ctx context.Context) MapSetting {
	return func(mo *mapOptions) {
		mo.ctx = ctx
	}
}

// Map takes a slice and a function, it then calls the function with each value of the slice
// The return of each function will be values in the returned slice
func Map[TYPE any, RET any](
	ss []TYPE,
	fn func(TYPE) (RET, error),
	settings ...MapSetting,
) ([]RET, error) {
	options := mapOptions{
		maxConcurrency: len(ss),
		ctx:            context.Background(),
	}
	for _, setting := range settings {
		setting(&options)
	}

	// Sanity checks
	if options.maxConcurrency > len(ss) {
		options.maxConcurrency = len(ss)
	} else if options.maxConcurrency < 0 {
		return nil, fmt.Errorf("maxConcurrency can't be less than 1, was %d", options.maxConcurrency)
	}

	// Setting up errors, so that new errors can be listened on with errChan, and they can be
	// set by calling `setErr(err)` any number of times, but the first one will only be used
	errChan := make(chan error, 1)
	errOnce := &sync.Once{}
	setErr := func(err error) {
		errOnce.Do(func() {
			errChan <- err
		})
	}
	defer close(errChan)

	// processingIndex is channel with the number
	processingIndex := make(chan int, options.maxConcurrency)
	defer close(processingIndex)

	wgDone, wgWait, wgStop := chanWaitGroup(len(ss))
	defer wgStop()

	ctx, _ := context.WithCancel(options.ctx)

	ret := make([]RET, len(ss))

	// Start up worked go-routines that will read from the work-pool and run the function with the value grabbed
	for i := 0; i < options.maxConcurrency; i++ {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					setErr(fmt.Errorf("panic: %v", err))
					wgDone()
				}
			}()

			// Fetch data from the data channel until nothing is left
			for i := range processingIndex {
				r, err := fn(ss[i])
				if err != nil {
					setErr(err)
				} else {
					ret[i] = r
				}
				wgDone()
			}
		}()
	}

	// Loop through all elements and put them into the queue, while
	for i := range ss {
		select {
		case err := <-errChan:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		case processingIndex <- i:
			// Job processed, continue to the next index
		}
	}

	// We now have started the last concurent go-routine

	// Wait for either all the final go-routines to finish, an error, or context cancellation
	select {
	case <-wgWait:
		return ret, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
