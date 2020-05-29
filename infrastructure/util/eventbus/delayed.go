package eventbus

import (
	"DDD/infrastructure/config"
	"DDD/infrastructure/util/mq/rabbitmq"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	delayedMaxTime int64 = 4147200
)

type delayed struct {
}

func (d *delayed) Publish(event *mqBusEvent) error {
	exchange := fmt.Sprintf("%s:delay:exchange", event.source)
	rm := &rabbitmq.RabbitMQ{
		URL:      viper.GetString("rabbitMq.url"),
		Exchange: exchange,
	}
	var t int64
	h, ok := event.headers["x-delay"]
	if ok {
		t = h.(int64)
	}
	if t > delayedMaxTime {
		t = delayedMaxTime
	}
	config.Logger.Info("print-srv:event-bus",
		zap.Any("source", event.source),
		zap.Any("exchange", exchange),
		zap.Any("end-time", t),
	)
	if err := rm.Load(); err != nil {
		return err
	}
	defer rm.Destroy()
	if err := rm.PublishWithDelay(event.source, []byte(event.data), t); err != nil {
		return err
	}
	return nil
}
