package playback

import (
	"fmt"
	"log"
	"time"
)

const (
	TIMER_PLAY = iota
	TIMER_PAUSE
	TIMER_STOP
)

type TimerCallback func(int)

// Timer keeps track of playback time
type Timer struct {
	time     int
	status   int
	callback TimerCallback
	timeChan chan int
}

func (t *Timer) Play() error {
	if t.timeChan == nil {
		panic("attempt to start a nil timer channel")
	}

	if t.status == TIMER_PLAY {
		log.Printf("STREAM PLAYBACK TIMER attempt to play an already playing timer, ignoring...")
		return fmt.Errorf("already playing")
	}

	t.status = TIMER_PLAY
	go Increment(t, t.timeChan)
	return nil
}

func (t *Timer) Stop() error {
	if t.timeChan == nil {
		panic("attempt to stop a nil timer channel")
	}

	t.time = 0
	if t.status != TIMER_PLAY {
		return nil
	}

	t.status = TIMER_STOP
	t.timeChan <- TIMER_STOP
	return nil
}

func (t *Timer) Pause() error {
	if t.timeChan == nil {
		panic("attempt to pause a nil timer channel")
	}

	if t.status != TIMER_PLAY {
		return nil
	}

	t.status = TIMER_PAUSE
	t.timeChan <- TIMER_PAUSE
	return nil
}

func (t *Timer) Set(time int) error {
	if time < 0 {
		return fmt.Errorf("time must be a positive integer")
	}

	t.time = time
	return nil
}

func (t *Timer) OnTick(callback TimerCallback) {
	t.callback = callback
}

func (t *Timer) GetTime() int {
	return t.time
}

func (t *Timer) GetStatus() int {
	return t.status
}

// Increment is a convenience function for incrementing
// a timer's time value every second. If a timer.callback
// func exists, it is called every increment interval.
// Warning: this is not a concurrency-safe function
func Increment(timer *Timer, c chan int) {
	if timer == nil {
		panic("attempt to increment a nil timer")
	}

	time.Sleep(time.Duration(1 * time.Second))
	timer.time++

	if timer.callback != nil {
		timer.callback(timer.time)
	}

	select {
	case sig := <-c:
		if sig == TIMER_PAUSE || sig == TIMER_STOP {
			log.Printf("STREAM PLAYBACK TIMER kill signal received: %v", sig)
			return
		}

		log.Printf("STREAM PLAYBACK TIMER invalid timer signal code: %v is not a recognized channel operation", sig)
	default:
		Increment(timer, c)
	}
}

func NewTimer() *Timer {
	return &Timer{
		status:   TIMER_STOP,
		timeChan: make(chan int),
	}
}
