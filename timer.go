package aevent

import "time"

type TimerCallback func(now time.Time)
type eventTimer struct {
	fireTime time.Time
	interval time.Duration
	callback TimerCallback
	repeat   bool
	timerId  uint64
}

func (t *eventTimer) Cancel() {
	t.callback = nil
}

func (t *eventTimer) IsActive() bool {
	return t.callback != nil
}

type timerHeap struct {
	timers []*eventTimer
}

func (h *timerHeap) Len() int {
	return len(h.timers)
}

func (h *timerHeap) Top() *eventTimer {
	if h.Len() == 0 {
		return nil
	}
	return h.timers[0]
}

func (h *timerHeap) Less(i, j int) bool {
	t1, t2 := h.timers[i].fireTime, h.timers[j].fireTime
	if t1.Before(t2) {
		return true
	}
	if t1.After(t2) {
		return false
	}
	// t1 == t2, making sure Timer with same deadline is fired according to their add order
	return h.timers[i].timerId < h.timers[j].timerId
}

func (h *timerHeap) Swap(i, j int) {
	tmp := h.timers[i]
	h.timers[i] = h.timers[j]
	h.timers[j] = tmp
}

func (h *timerHeap) Push(x interface{}) {
	h.timers = append(h.timers, x.(*eventTimer))
}

func (h *timerHeap) Pop() (ret interface{}) {
	l := len(h.timers)
	h.timers, ret = h.timers[:l-1], h.timers[l-1]
	return
}
