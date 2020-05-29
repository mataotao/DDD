package eventbus

import (
	"DDD/infrastructure/util/pkg/snowflake"
	"errors"
	"time"
)

const (
	_ int8 = iota
	EventStreams
	EventRabbitMqDelay
)

var MqBusMq = map[int8]Mq{
	EventStreams: Mq(new(streams)),
	//EventRabbitMqDelay: delayed,
}

type Mq interface {
	Publish(event *mqBusEvent) error
}

type MqBusPublisher interface {
	Publish(eventType int8, topic, args string) error
}

type MqBus interface {
	MqBusPublisher
}

type mqBusEvent struct {
	id       string
	datetime string
	source   string
	data     string
}
type MessageQueueBus struct {
}

func (bus *MessageQueueBus) Publish(eventType int8, topic, args string) error {
	f, ok := MqBusMq[eventType]
	if !ok {
		return errors.New("事件类型错误")
	}
	event := &mqBusEvent{
		id:       snowflake.BaseNumber(),
		datetime: time.Now().Format("2006-01-02 15:04:05"),
		source:   topic,
		data:     args,
	}

	return f.Publish(event)
}
func NewMqBus() MqBus {
	return MqBus(new(MessageQueueBus))
}
