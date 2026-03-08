package zpub

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"reflect"
	"sync"
)

type EventBus struct {
	obServers map[string]map[ObServer]struct{}
	lock      sync.RWMutex
}

var EventSystem *EventBus

func init() {
	EventSystem = NewEventBus()
}

func NewEventBus() *EventBus {
	return &EventBus{
		obServers: make(map[string]map[ObServer]struct{}),
	}

}

func (e *EventBus) Subscribe(topic string, o ObServer) {
	val := reflect.ValueOf(o)
	if val.Kind() != reflect.Ptr {
		// 直接 Panic，让开发者在开发阶段就发现问题
		panic(fmt.Sprintf("EventBus: Subscribe observer must be a pointer, got %T", o))
	}
	e.lock.Lock()
	defer e.lock.Unlock()
	if _, ok := e.obServers[topic]; !ok {
		e.obServers[topic] = make(map[ObServer]struct{})
	}
	e.obServers[topic][o] = struct{}{}
}

func (e *EventBus) UnSubscribe(topic string, o ObServer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if _, ok := e.obServers[topic]; !ok {
		return
	}
	delete(e.obServers[topic], o)
	if len(e.obServers[topic]) == 0 {
		delete(e.obServers, topic)
	}
}

func (e *EventBus) Publish(event *Event) {
	if event == nil {
		return
	}
	e.lock.RLock()
	originalMap, ok := e.obServers[event.Topic]
	if !ok || len(originalMap) == 0 {
		e.lock.RUnlock()
		return
	}
	subscribers := make([]ObServer, 0, len(originalMap))
	for o := range originalMap {
		subscribers = append(subscribers, o)
	}
	e.lock.RUnlock()

	for _, o := range subscribers {
		e.safeCall(o, event)
	}
}

func (e *EventBus) AsyncPublish(event *Event) {
	e.lock.RLock()
	originalMap, ok := e.obServers[event.Topic]
	if !ok || len(originalMap) == 0 {
		e.lock.RUnlock()
		return
	}
	subscribers := make([]ObServer, 0, len(originalMap))
	for o := range originalMap {
		subscribers = append(subscribers, o)
	}
	e.lock.RUnlock()
	for _, o := range subscribers {
		ob := o
		go func() {
			defer zlog.Recover("EventBus: AsyncPublish panic recovered")
			ob.OnChange(event)
		}()
	}
}
func (e *EventBus) safeCall(o ObServer, event *Event) {
	defer zlog.Recover("EventBus: observer panic recovered")
	o.OnChange(event)
}
