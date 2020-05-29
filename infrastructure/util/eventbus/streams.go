package eventbus

import (
	"DDD/infrastructure/config"
	"DDD/infrastructure/util/redis"

	"go.uber.org/zap"
)

const (
	streamsMaxLen uint64 = 1000000 //streams 最大长度
)

type streams struct {
}

func (s *streams) Publish(event *mqBusEvent) error {
	data := []redis.StreamValue{
		redis.StreamValue{
			Field: "id",
			Value: event.id,
		},
		redis.StreamValue{
			Field: "datetime",
			Value: event.datetime,
		},
		redis.StreamValue{
			Field: "data",
			Value: event.data,
		},
	}
	client := redis.NewClient(redis.Pool.Get())
	defer client.Close()
	config.Logger.Info("print-srv:event-bus",
		zap.Any("source", event.source),
		zap.Any("data", data),
	)
	if err := client.XAdd(event.source, "*", streamsMaxLen, data...); err != nil {
		config.Logger.Error("Error", zap.Error(err))
		return err
	}
	return nil
}
