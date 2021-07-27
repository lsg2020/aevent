package main

import (
	"context"
	"log"
	"time"

	"github.com/lsg2020/aevent"
)

const (
	EventNameCall      = "EventNameCall"
	EventNameCallSleep = "EventNameCallSleep"
)

type EventData struct {
	id     uint32
	amount uint32
	sleep  time.Duration
}

func main() {
	// 创建管理器
	event := aevent.NewEventManager()

	// 注册
	event.RegisterEvent(EventNameCall, func(event *aevent.Event) (interface{}, error) {
		ev := event.Data().(*EventData)
		log.Println("process call ", ev)
		return ev, nil
	})
	event.RegisterEventRaw(EventNameCallSleep, func(event *aevent.Event) {
		ev := event.Data().(*EventData)
		go func() {
			log.Println("process begin call sleep", ev)
			time.Sleep(ev.sleep)
			log.Println("process end call sleep ", ev)

			event.Response(ev, nil)
		}()
	})

	go func() {
		// call event
		{
			ctx, _ := context.WithTimeout(context.Background(), time.Second*1)
			rsp, err := event.CallEvent(ctx, EventNameCall, &EventData{
				id:     1,
				amount: 2,
			})
			log.Println("call result", rsp, err)
		}

		// call event response in other goroutine
		{
			ctx, _ := context.WithTimeout(context.Background(), time.Second*1)
			rsp, err := event.CallEvent(ctx, EventNameCallSleep, &EventData{
				id:     3,
				amount: 4,
				sleep:  time.Microsecond * 500,
			})
			log.Println("call sleep result", rsp, err)
		}

		// call timeout
		{
			ctx, _ := context.WithTimeout(context.Background(), time.Second*1)
			rsp, err := event.CallEvent(ctx, EventNameCallSleep, &EventData{
				id:     3,
				amount: 4,
				sleep:  time.Second * 2,
			})
			log.Println("call sleep result", rsp, err)
		}

	}()

	event.SetInterval(time.Second*2, func(now time.Time) {
		log.Println("on timer")
	})
	event.Exec(func() {
		log.Println("on exec call")
	})

	log.Println("start event serve")

	// 处理事件
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	_ = event.Serve(ctx)
}
