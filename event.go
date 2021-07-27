package aevent

import (
	"github.com/Workiva/go-datastructures/queue"
)

const (
	EventTypeEvent = iota
	EventTypeExec
	EventTypeTimer
	EventTypeExit
)

type Event struct {
	eventType uint32

	name     string
	data     interface{}
	response *queue.RingBuffer

	exec func()
}

func newEvent(name string, data interface{}, needResponse bool) *Event {
	ev := &Event{
		eventType: EventTypeEvent,
		name:      name,
		data:      data,
	}
	if needResponse {
		ev.response = queue.NewRingBuffer(1)
	}
	return ev
}

func newTimerEvent() *Event {
	return &Event{
		eventType: EventTypeTimer,
	}
}

func newExecEvent(cb func()) *Event {
	return &Event{
		eventType: EventTypeExec,
		exec:      cb,
	}
}

func newExitEvent() *Event {
	return &Event{
		eventType: EventTypeExit,
	}
}

func (e Event) Name() string {
	return e.name
}

func (e *Event) Data() interface{} {
	return e.data
}

func (e *Event) Response(response interface{}, err error) {
	if err != nil {
		e.response.Put(err)
	} else {
		e.response.Put(response)
	}
}
