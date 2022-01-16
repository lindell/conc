package conc

import "sync"

// chanWaitGroup works as a waitgroup, but wait is instead a channel that will be closed when the counter hits 0
// This has the benefit of being able to `select` on that channel together with other conditions
// `done` is called to decrease the counter
// `waitCh` is closed when the counter hits zero
// `stop` will stop the listening and clean up the go-routine, preferable run with `defer stop()`
func chanWaitGroup(size int) (done func(), waitCh chan struct{}, stop func()) {
	doneCh := make(chan struct{}, size)
	waitCh = make(chan struct{}, 1)
	left := size

	stopped := false
	stopLock := sync.RWMutex{}

	done = func() {
		stopLock.RLock()
		defer stopLock.RUnlock()
		if !stopped {
			doneCh <- struct{}{}
		}
	}

	stop = func() {
		stopLock.Lock()
		defer stopLock.Unlock()
		if !stopped {
			stopped = true
			close(doneCh)
		}
	}

	go func() {
		for range doneCh {
			left--
			if left == 0 {
				close(waitCh)
				stop()
				return
			}
		}
		close(waitCh)
	}()

	return done, waitCh, stop
}
