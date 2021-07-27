package aevent

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Workiva/go-datastructures/queue"
)

type EventCallback func(*Event) (interface{}, error)

type EventManager struct {
	dispatcher     map[string]func(*Event)
	workRingBuffer *queue.RingBuffer
	workMutex      sync.Mutex
	nextTimerId    uint64
	timerHeap      timerHeap
	timer          *time.Timer
}

func NewEventManager() *EventManager {
	mgr := &EventManager{
		workRingBuffer: queue.NewRingBuffer(1024),
		dispatcher:     make(map[string]func(*Event)),
		nextTimerId:    1,
	}
	heap.Init(&mgr.timerHeap)
	return mgr
}

func (mgr *EventManager) Serve(ctx context.Context) error {
	for {
		workCtx, workCancel := context.WithCancel(context.Background())
		go mgr.woker(workCancel)

		select {
		case <-workCtx.Done():
			continue
		case <-ctx.Done():
			ev := newExitEvent()
			mgr.pushEvent(ev)
			return ctx.Err()
		}
	}
}

func (mgr *EventManager) woker(cancel context.CancelFunc) {
	for {
		e, err := mgr.workRingBuffer.Get()
		if err != nil {
			cancel()
			log.Println("aevent work error", err)
			return
		}

		event := e.(*Event)
		switch event.eventType {
		case EventTypeEvent:
			mgr.handleEvent(event, cancel)
		case EventTypeTimer:
			mgr.handleTimer(cancel)
		case EventTypeExec:
			mgr.handleExec(event.exec, cancel)
		case EventTypeExit:
			return
		}
	}
}

func (mgr *EventManager) handleEvent(ev *Event, cancel context.CancelFunc) {
	if cb, ok := mgr.dispatcher[ev.Name()]; ok {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("async event event:%s recover panic %v\n", ev.Name(), r)
				ev.Response(nil, fmt.Errorf("%v", r))
				debug.PrintStack()
				cancel()
			}
		}()

		cb(ev)
	} else {
		log.Printf("async event event:%s callback not exists", ev.Name())
	}
}

func (mgr *EventManager) handleExec(cb func(), cancel context.CancelFunc) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("async event exec recover panic %v\n", r)
			debug.PrintStack()
			cancel()
		}
	}()

	cb()
}

func (mgr *EventManager) handleTimer(cancel context.CancelFunc) {
	now := time.Now()
	var repeatTimer []*eventTimer
	for {
		if mgr.timerHeap.Len() == 0 {
			break
		}

		top := mgr.timerHeap.Top()
		if top.fireTime.After(now) {
			break
		}

		t := heap.Pop(&mgr.timerHeap).(*eventTimer)
		cb := t.callback
		if cb == nil {
			continue
		}
		if !t.repeat {
			t.callback = nil
		} else {
			t.fireTime = t.fireTime.Add(t.interval)
			repeatTimer = append(repeatTimer, t)
		}

		// callback
		mgr.onTimer(cb, now, cancel, repeatTimer)
	}

	for _, t := range repeatTimer {
		heap.Push(&mgr.timerHeap, t)
	}
	if mgr.timerHeap.Len() != 0 {
		mgr.fireTimer(nil, mgr.timerHeap.Top())
	}
}

func (mgr *EventManager) onTimer(cb TimerCallback, now time.Time, cancel context.CancelFunc, repeatTimer []*eventTimer) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("async event timer recover panic %v\n", r)
			debug.PrintStack()
			cancel()

			for _, t := range repeatTimer {
				heap.Push(&mgr.timerHeap, t)
			}
			if mgr.timerHeap.Len() != 0 {
				mgr.fireTimer(nil, mgr.timerHeap.Top())
			}
		}
		cancel()
	}()

	cb(now)
}

func (mgr *EventManager) RegisterEvent(name string, cb EventCallback) {
	mgr.dispatcher[name] = func(ev *Event) {
		rsp, err := cb(ev)
		if ev.response != nil {
			ev.Response(rsp, err)
		}
	}
}

func (mgr *EventManager) RegisterEventRaw(name string, cb func(*Event)) {
	mgr.dispatcher[name] = cb
}

func (mgr *EventManager) pushEvent(e *Event) {
	mgr.workMutex.Lock()
	defer mgr.workMutex.Unlock()
	mgr.workRingBuffer.Put(e)

}

func (mgr *EventManager) CallEvent(ctx context.Context, name string, msg interface{}) (rsp interface{}, err error) {
	ev := newEvent(name, msg, true)
	mgr.pushEvent(ev)

	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		default:
		}

		rspVal, rspErr := ev.response.Poll(time.Second)
		if rspErr == queue.ErrTimeout {
			continue
		} else if rspErr != nil {
			err = rspErr
			return
		}

		var ok bool
		if err, ok = rspVal.(error); ok {
			return
		} else {
			rsp = rspVal
			return
		}
	}
}

func (mgr *EventManager) SendEvent(name string, msg interface{}) {
	ev := newEvent(name, msg, false)
	mgr.pushEvent(ev)
}

func (mgr *EventManager) Exec(cb func()) {
	ev := newExecEvent(cb)
	mgr.pushEvent(ev)
}

func (mgr *EventManager) fireTimer(last *eventTimer, cur *eventTimer) {
	if last != nil && cur != nil && last.fireTime.Before(cur.fireTime) {
		return
	}

	if cur == nil {
		return
	}
	if mgr.timer != nil {
		mgr.timer.Stop()
	}
	now := time.Now()
	mgr.timer = time.AfterFunc(cur.fireTime.Sub(now), func() {
		ev := newTimerEvent()
		mgr.pushEvent(ev)
	})
}

func (mgr *EventManager) SetTimeout(d time.Duration, cb TimerCallback) {
	t := &eventTimer{
		fireTime: time.Now().Add(d),
		interval: d,
		callback: cb,
		repeat:   false,
		timerId:  mgr.nextTimerId,
	}
	mgr.nextTimerId++

	last := mgr.timerHeap.Top()
	heap.Push(&mgr.timerHeap, t)
	mgr.fireTimer(last, t)
}

func (mgr *EventManager) SetInterval(d time.Duration, cb TimerCallback) {
	t := &eventTimer{
		fireTime: time.Now().Add(d),
		interval: d,
		callback: cb,
		repeat:   true,
		timerId:  mgr.nextTimerId,
	}
	mgr.nextTimerId++

	last := mgr.timerHeap.Top()
	heap.Push(&mgr.timerHeap, t)
	mgr.fireTimer(last, t)
}
