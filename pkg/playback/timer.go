package playback

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
)

const (
	MAX_TIMER_CHAN_BUFFER = 1

	TIMER_PLAY = iota
	TIMER_PAUSE
	TIMER_STOP
)

type TimerCallback func(int)

// Timer keeps track of playback time
type Timer struct {
	time      int
	state     int
	callbacks []TimerCallback
	timeChan  chan int
}

func (t *Timer) Play() error {
	if t.timeChan == nil {
		panic("attempt to start a nil timer channel")
	}

	if t.state == TIMER_PLAY {
		log.Printf("STREAM PLAYBACK TIMER attempt to play an already playing timer, ignoring...")
		return nil
	}

	t.state = TIMER_PLAY
	go Increment(t, t.timeChan)
	return nil
}

func (t *Timer) Stop() error {
	if t.timeChan == nil {
		panic("attempt to stop a nil timer channel")
	}

	t.time = 0
	if t.state != TIMER_PLAY {
		return nil
	}

	t.state = TIMER_STOP
	t.timeChan <- TIMER_STOP
	return nil
}

func (t *Timer) Pause() error {
	if t.timeChan == nil {
		panic("attempt to pause a nil timer channel")
	}

	if t.state != TIMER_PLAY {
		return nil
	}

	t.state = TIMER_PAUSE
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
	t.callbacks = append(t.callbacks, callback)
}

func (t *Timer) GetTime() int {
	return t.time
}

func (t *Timer) State() int {
	return t.state
}

// TimerStatus is a serializable schema representing a summary of
// the current state of the Timer.
type TimerStatus struct {
	IsPlaying bool `json:"isPlaying"`
	IsPaused  bool `json:"isPaused"`
	IsStopped bool `json:"isStopped"`
	Time      int  `json:"time"`
}

func (s *TimerStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (t *Timer) Status() api.ApiCodec {
	return &TimerStatus{
		IsPlaying: t.state == TIMER_PLAY,
		IsStopped: t.state == TIMER_STOP,
		IsPaused:  t.state == TIMER_PAUSE,
		Time:      t.time,
	}
}

// Increment is a convenience function for incrementing
// a timer's time value every second. If a timer.callback
// func exists, it is called every increment interval.
// Warning: this is not a concurrency-safe function
func Increment(timer *Timer, c chan int) {
	if timer == nil {
		panic("attempt to increment a nil timer")
	}

	for {
		time.Sleep(time.Duration(1 * time.Second))
		timer.time++

		if len(timer.callbacks) > 0 {
			for _, c := range timer.callbacks {
				c(timer.time)
			}
		}

		select {
		case sig := <-c:
			if sig == TIMER_PAUSE || sig == TIMER_STOP {
				log.Printf("STREAM PLAYBACK TIMER kill signal received: %v", sig)
				return
			}

			log.Printf("STREAM PLAYBACK TIMER invalid timer signal code: %v is not a recognized channel operation", sig)
		default:
			continue
		}
	}
}

func NewTimer() *Timer {
	return &Timer{
		state:     TIMER_STOP,
		timeChan:  make(chan int, MAX_TIMER_CHAN_BUFFER),
		callbacks: []TimerCallback{},
	}
}
