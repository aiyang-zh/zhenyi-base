package zpub

type ObServer interface {
	OnChange(e *Event)
}

type IEventBus interface {
	Subscribe(topic string, server ObServer)
	UnSubscribe(topic string, server ObServer)
	Publish(event *Event)
	AsyncPublish(event *Event)
}

type Event struct {
	Topic string
	Val   interface{}
}
