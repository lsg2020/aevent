package aevent

type Event struct {
	name string
	data interface{}

	rspChan chan interface{}
	errChan chan error
}

func newEvent(name string, data interface{}, needResponse bool) *Event {
	ev := &Event{
		name: name,
		data: data,
	}
	if needResponse {
		ev.rspChan = make(chan interface{}, 1)
		ev.errChan = make(chan error, 1)
	}
	return ev
}

func (e Event) Name() string {
	return e.name
}

func (e *Event) Data() interface{} {
	return e.data
}

func (e *Event) Response() (chan interface{}, chan error) {
	return e.rspChan, e.errChan
}
