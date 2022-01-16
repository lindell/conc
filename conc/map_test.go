package conc_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lindell/conc/conc"
	"github.com/stretchr/testify/assert"
)

const bigTestSize = 10000

// Time to wait after test to ensure that go all go routines has finished
var finishWait = time.Millisecond * 100

func init() {
	if waitString, ok := os.LookupEnv("CONC_FINISH_WAIT"); ok {
		if wait, err := time.ParseDuration(waitString); err == nil {
			finishWait = wait
		}
	}
}

// By running `defer checkGoRoutines(t)()` the test will check that there is no go routines
func checkGoRoutines(t *testing.T) func() {
	before := runtime.NumGoroutine()
	return func() {
		time.Sleep(finishWait)
		if after := runtime.NumGoroutine(); after > before {
			t.Fatalf("number of go routines has increased, was %d, is %d", before, after)
		}
	}
}

func TestMap(t *testing.T) {
	ret, err := conc.Map([]string{"6", "2", "1", "76"}, strconv.Atoi)
	assert.NoError(t, err)
	assert.Equal(t, []int{6, 2, 1, 76}, ret)
}

func TestMapMaxConcurrency(t *testing.T) {
	defer checkGoRoutines(t)()

	const concurrent = 2
	latest := 0
	lock := sync.Mutex{}

	ints := make([]int, bigTestSize)
	for i := 0; i < bigTestSize; i++ {
		ints[i] = i
	}
	_, err := conc.Map(ints, func(v int) (string, error) {
		lock.Lock()
		defer lock.Unlock()
		if v > latest+concurrent {
			t.Fatal("jumped ahead more than the max concurrency")
		} else if v > latest {
			latest = v
		}
		return fmt.Sprint(v), nil
	}, conc.WithMaxConcurrency(concurrent))
	assert.NoError(t, err)
}

func TestAllErrors(t *testing.T) {
	defer checkGoRoutines(t)()

	calls := 0
	lock := sync.Mutex{}

	ints := make([]int, bigTestSize)
	_, err := conc.Map(ints, func(v int) (string, error) {
		time.Sleep(time.Millisecond)
		lock.Lock()
		defer lock.Unlock()
		calls++
		return "", errors.New("test error")
	}, conc.WithMaxConcurrency(10))
	assert.Equal(t, errors.New("test error"), err)
	time.Sleep(finishWait)
	lock.Lock()
	defer lock.Unlock()
	assert.LessOrEqual(t, calls, 20) // There might be some time from the error being detected to it being returned
}

func TestOneErrors(t *testing.T) {
	defer checkGoRoutines(t)()

	ints := make([]int, bigTestSize)
	ints[len(ints)-4] = 1
	_, err := conc.Map(ints, func(v int) (string, error) {
		if v == 1 {
			return "", errors.New("test error")
		}
		return "", nil
	}, conc.WithMaxConcurrency(10))
	assert.Equal(t, errors.New("test error"), err)
}

func TestMapPanic(t *testing.T) {
	defer checkGoRoutines(t)()

	ints := make([]int, bigTestSize)
	ints[bigTestSize-2] = 1

	_, err := conc.Map(ints, func(v int) (int, error) {
		return 1 / (v - 1), nil // Panics if the value is 1
	}, conc.WithMaxConcurrency(10))
	assert.Equal(t, errors.New("panic: runtime error: integer divide by zero"), err)
}

func TestMapCancelContextEarly(t *testing.T) {
	defer checkGoRoutines(t)()

	ints := make([]int, bigTestSize)
	ctx, cancel := context.WithCancel(context.Background())

	ints[2] = 2
	ints[bigTestSize-2] = 1

	_, err := conc.Map(ints, func(v int) (int, error) {
		if v == 2 {
			cancel()
		}
		return 1 / (v - 1), nil // Panics if the value is 1
	}, conc.WithMaxConcurrency(10), conc.WithContext(ctx))
	assert.Equal(t, errors.New("context canceled"), err)
}

func TestMapCancelContextLate(t *testing.T) {
	defer checkGoRoutines(t)()

	const timeBeforeCancel = time.Millisecond * 10
	const timeLongestMap = time.Millisecond * 100

	ints := make([]int, bigTestSize)
	ctx, cancel := context.WithCancel(context.Background())

	ints[bigTestSize-2] = 1

	beforeTime := time.Now()
	go func() {
		time.Sleep(timeBeforeCancel)
		cancel()
	}()
	_, err := conc.Map(ints, func(v int) (int, error) {
		if v == 1 {
			time.Sleep(timeLongestMap)
		}
		return 0, nil
	}, conc.WithMaxConcurrency(10), conc.WithContext(ctx))
	assert.Equal(t, errors.New("context canceled"), err)

	assert.LessOrEqual(t, time.Since(beforeTime), timeLongestMap)
	time.Sleep(timeLongestMap - timeBeforeCancel) // Make sure we don't leave any goroutines behind
}
