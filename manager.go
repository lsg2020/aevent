package aevent

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"
)

type EventCallback func(*Event) (interface{}, error)

type EventManager struct {
	dispatcher  map[string]func(*Event)
	eventChan   chan *Event
	execChan    chan func()
	timerChan   chan struct{}
	nextTimerId uint64
	timerHeap   timerHeap
	timer       *time.Timer
}

func NewEventManager() *EventManager {
	mgr := &EventManager{
		eventChan:   make(chan *Event, 1024),
		dispatcher:  make(map[string]func(*Event)),
		timerChan:   make(chan struct{}, 1024),
		execChan:    make(chan func(), 1024),
		nextTimerId: 1,
	}
	heap.Init(&mgr.timerHeap)
	return mgr
}

func (mgr *EventManager) Serve(ctx context.Context) error {
	for {
		workCtx, workCancel := context.WithCancel(context.Background())
		go mgr.woker(ctx, workCancel)

		select {
		case <-workCtx.Done():
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (mgr *EventManager) woker(ctx context.Context, cancel context.CancelFunc) {
	for {
		select {
		case ev := <-mgr.eventChan:
			mgr.handleEvent(ev, cancel)
		case cb := <-mgr.execChan:
			mgr.handleExec(cb, cancel)
		case <-mgr.timerChan:
			mgr.handleTimer(cancel)
		case <-ctx.Done():
			return
		}
	}
}

func (mgr *EventManager) handleEvent(ev *Event, cancel context.CancelFunc) {
	if cb, ok := mgr.dispatcher[ev.Name()]; ok {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("async event event:%s recover panic %v\n", ev.Name(), r)
				if ev.errChan != nil {
					ev.errChan <- fmt.Errorf("%v", r)
				}
				debug.PrintStack()
				cancel()
			}
		}()

		cb(ev)
		/*
			rsp, err := cb(ev)
			if ev.rspChan != nil && ev.errChan != nil {
				if err != nil {
					ev.errChan <- err
				} else {
					ev.rspChan <- rsp
				}
			}
		*/
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
		if ev.rspChan != nil && ev.errChan != nil {
			if err != nil {
				ev.errChan <- err
			} else {
				ev.rspChan <- rsp
			}
		}
	}
}

func (mgr *EventManager) RegisterEventRaw(name string, cb func(*Event)) {
	mgr.dispatcher[name] = cb
}

func (mgr *EventManager) CallEvent(ctx context.Context, name string, msg interface{}) (rsp interface{}, err error) {
	ev := newEvent(name, msg, true)
	mgr.eventChan <- ev

	select {
	case rsp = <-ev.rspChan:
		return
	case err = <-ev.errChan:
		return
	case <-ctx.Done():
		err = ctx.Err()
		return
	}
}

func (mgr *EventManager) SendEvent(name string, msg interface{}) {
	ev := newEvent(name, msg, false)
	mgr.eventChan <- ev
}

func (mgr *EventManager) Exec(cb func()) {
	mgr.execChan <- cb
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
	mgr.timer = time.AfterFunc(cur.fireTime.Sub(time.Now()), func() {
		mgr.timerChan <- struct{}{}
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
