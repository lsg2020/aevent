package main

import (
	"context"
	"flag"
	"log"
	"time"
	
	"github.com/lsg2020/aevent"
)

var workAmount = flag.Int("work_amount", 30, "")
var seconds = flag.Duration("seconds", 10*time.Second, "")

const EventName = "TestEvent"

type EventData struct {
	v1 uint32
	v2 string
	v3 []uint32
}

func main() {
	flag.Parse()

	event := aevent.NewEventManager()

	var eventTotal uint32 = 0
	eventProcessAmount := 0
	eventLastTime := time.Now().UnixNano()
	event.RegisterEvent(EventName, func(data *aevent.Event) (interface{}, error) {
		req := data.Data().(*EventData)
		eventTotal = eventTotal + req.v1

		eventProcessAmount++
		return data.Data(), nil
	})

	event.SetInterval(time.Second, func(now time.Time) {
		nowNano := time.Now().UnixNano()
		log.Printf("time: %.2f seconds process amount: %d", float64(nowNano-eventLastTime)/float64(time.Second.Nanoseconds()), eventProcessAmount)
		eventProcessAmount = 0
		eventLastTime = nowNano
	})

	ctx, cancel := context.WithTimeout(context.Background(), *seconds)
	defer cancel()

	for i := 0; i < *workAmount; i++ {
		go func(ctx context.Context, event *aevent.EventManager) {

			v1 := uint32(i * 1000)
			for {
				data := &EventData{
					v1: v1,
					v2: "4321",
					v3: []uint32{1, 2, 3, 4},
				}
				v1++

				rsp, err := event.CallEvent(ctx, EventName, data)
				if err != nil {
					log.Fatalln("call event error", err)
				}

				ret := rsp.(*EventData)
				if ret.v1 != data.v1 {
					log.Fatalln("call event response error")
				}
			}
		}(ctx, event)
	}

	_ = event.Serve(ctx)
}
