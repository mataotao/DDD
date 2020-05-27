package eventbus

import (
	"fmt"
	"reflect"
	"sync"
)

//BusSubscriber 订阅
type BusSubscriber interface {
	Subscribe(topic string, fn interface{}) error
	SubscribeAsync(topic string, fn interface{}, transactional bool) error
	SubscribeOnce(topic string, fn interface{}) error
	SubscribeOnceAsync(topic string, fn interface{}) error
	Unsubscribe(topic string, handler interface{}) error
}

//BusPublisher 发布
type BusPublisher interface {
	Publish(topic string, args ...interface{})
}

//BusController 检查
type BusController interface {
	HasCallback(topic string) bool
	WaitAsync()
}


//Bus 总线
type Bus interface {
	BusController
	BusSubscriber
	BusPublisher
}

// EventBus 事件总线
type EventBus struct {
	handlers *sync.Map
	wg       *sync.WaitGroup
}

type eventHandler struct {
	callBack      reflect.Value
	flagOnce      bool
	async         bool
	transactional bool
	*sync.Mutex
}

// New new
func New() Bus {
	b := &EventBus{
		new(sync.Map),
		new(sync.WaitGroup),
	}
	return Bus(b)
}

// doSubscribe 处理订阅逻辑
func (bus *EventBus) doSubscribe(topic string, fn interface{}, handler *eventHandler) error {
	if !(reflect.TypeOf(fn).Kind() == reflect.Func) {
		return fmt.Errorf("%s is not of type reflect.Func", reflect.TypeOf(fn).Kind())
	}
	handlerInterface, _ := bus.handlers.LoadOrStore(topic, make([]*eventHandler, 0))
	handlers := handlerInterface.([]*eventHandler)
	bus.handlers.Store(topic, append(handlers, handler))
	return nil
}

// Subscribe 订阅-同步
func (bus *EventBus) Subscribe(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), false, false, false, new(sync.Mutex),
	})
}

// SubscribeAsync  订阅-异步
func (bus *EventBus) SubscribeAsync(topic string, fn interface{}, transactional bool) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), false, true, transactional, new(sync.Mutex),
	})
}

// SubscribeOnce 订阅-只执行一次-同步
func (bus *EventBus) SubscribeOnce(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), true, false, false, new(sync.Mutex),
	})
}

// SubscribeOnceAsync 订阅-只执行一次-异步
func (bus *EventBus) SubscribeOnceAsync(topic string, fn interface{}) error {
	return bus.doSubscribe(topic, fn, &eventHandler{
		reflect.ValueOf(fn), true, true, false, new(sync.Mutex),
	})
}

// HasCallback 查看事件订阅的函数
func (bus *EventBus) HasCallback(topic string) bool {
	handlersInterface, ok := bus.handlers.Load(topic)
	if ok {
		handlers := handlersInterface.([]*eventHandler)
		return len(handlers) > 0
	}
	return false
}

// Unsubscribe 删除订阅
func (bus *EventBus) Unsubscribe(topic string, handler interface{}) error {

	if handlersInterface, ok := bus.handlers.Load(topic); ok {
		handlers := handlersInterface.([]*eventHandler)
		if len(handlers) > 0 {
			bus.removeHandler(topic, bus.findHandlerIdx(topic, reflect.ValueOf(handler)))
		}
		return nil
	}
	return fmt.Errorf("topic %s doesn't exist", topic)
}

// Publish 推送
func (bus *EventBus) Publish(topic string, args ...interface{}) {

	if handlerInterface, ok := bus.handlers.Load(topic); ok {
		handlers := handlerInterface.([]*eventHandler)
		if len(handlers) == 0 {
			return
		}

		for i := range handlers[:] {
			if handlers[i].flagOnce {
				bus.removeHandler(topic, i)
			}
			if !handlers[i].async {
				bus.doPublish(handlers[i], topic, args...)
			} else {
				bus.wg.Add(1)
				if handlers[i].transactional {
					handlers[i].Lock()
				}
				go bus.doPublishAsync(handlers[i], topic, args...)
			}
		}
	}
}

func (bus *EventBus) doPublish(handler *eventHandler, topic string, args ...interface{}) {
	passedArguments := bus.setUpPublish(handler, args...)
	handler.callBack.Call(passedArguments)
}

func (bus *EventBus) doPublishAsync(handler *eventHandler, topic string, args ...interface{}) {
	defer bus.wg.Done()
	if handler.transactional {
		defer handler.Unlock()
	}
	bus.doPublish(handler, topic, args...)
}

func (bus *EventBus) removeHandler(topic string, idx int) {
	handlerInterface, ok := bus.handlers.Load(topic)
	if !ok {
		return
	}
	handlers := handlerInterface.([]*eventHandler)
	l := len(handlers)

	if !(0 <= idx && idx < l) {
		return
	}
	handlers = append(handlers[:idx], handlers[idx+1:]...)
	if len(handlers) > 0 {
		bus.handlers.Store(topic, handlers)
	} else {
		bus.handlers.Delete(topic)
	}

}

func (bus *EventBus) findHandlerIdx(topic string, callback reflect.Value) int {

	if handlerInterface, ok := bus.handlers.Load(topic); ok {
		handlers := handlerInterface.([]*eventHandler)
		for i := range handlers[:] {
			if handlers[i].callBack.Type() == callback.Type() &&
				handlers[i].callBack.Pointer() == callback.Pointer() {
				return i
			}
		}
	}
	return -1
}

func (bus *EventBus) setUpPublish(callback *eventHandler, args ...interface{}) []reflect.Value {
	funcType := callback.callBack.Type()
	passedArguments := make([]reflect.Value, len(args))
	for i := range args[:] {
		if args[i] == nil {
			passedArguments[i] = reflect.New(funcType.In(i)).Elem()
		} else {
			passedArguments[i] = reflect.ValueOf(args[i])
		}
	}

	return passedArguments
}

// WaitAsync 等待
func (bus *EventBus) WaitAsync() {
	bus.wg.Wait()
}
