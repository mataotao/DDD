package eventbus

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

//BusSubscriber 订阅
type BusSubscriber interface {
	Subscribe(observer interface{}) error
	SubscribeAsync(observer interface{}, transactional bool) error
	SubscribeOnce(observer interface{}) error
	SubscribeOnceAsync(observer interface{}) error
	Unsubscribe(observer interface{}) error
}

//BusPublisher 发布
type BusPublisher interface {
	Publish(topic string, args ...interface{})
}

//BusController 检查
type BusController interface {
	HasCallback(topic string) bool
	WaitAsync()
	Stop()
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
	observer      interface{}
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
func (bus *EventBus) doSubscribe(topic string, handler *eventHandler) error {
	handlerInterface, _ := bus.handlers.LoadOrStore(topic, make([]*eventHandler, 0))
	handlers := handlerInterface.([]*eventHandler)
	bus.handlers.Store(topic, append(handlers, handler))
	return nil
}

func (bus *EventBus) checkObserver(observer interface{}) ([]string, reflect.Type, string, error) {
	topic := make([]string, 0)
	var t reflect.Type
	var fn string
	var ok bool

	t = reflect.TypeOf(observer)
	if t.Kind() != reflect.Struct {
		return topic, t, fn, fmt.Errorf("%s is not of type reflect.Struct", t.Kind())
	}
	fnField, _ := t.FieldByName("Fn")
	if fnField.Tag == "" {
		return topic, t, fn, fmt.Errorf("%v has no field or no fn field", fnField)
	}
	fn, ok = fnField.Tag.Lookup("subscribe")
	if !ok || fn == "" {
		return topic, t, fn, fmt.Errorf("func %s doesn't exist or empty", "tag")
	}
	topics, ok := fnField.Tag.Lookup("topic")
	if !ok || topics == "" {
		return topic, t, fn, fmt.Errorf("topic %s doesn't exist or empty", "tag")
	}
	topic = strings.Split(topics, ",")
	return topic, t, fn, nil
}
func (bus *EventBus) Register(observer interface{}, flagOnce, async, transactional bool) error {
	topic, t, fn, err := bus.checkObserver(observer)
	if err != nil {
		return err
	}
	for i := range topic[:] {
		function, ok := t.MethodByName(fn)
		if !ok {
			continue
		}
		_ = bus.doSubscribe(topic[i], &eventHandler{
			observer, function.Func, flagOnce, async, transactional, new(sync.Mutex),
		})
	}
	return nil
}

// Subscribe 订阅-同步
func (bus *EventBus) Subscribe(observer interface{}) error {
	return bus.Register(observer, false, false, false)
}

// SubscribeAsync  订阅-异步
func (bus *EventBus) SubscribeAsync(observer interface{}, transactional bool) error {
	return bus.Register(observer, false, true, transactional)

}

// SubscribeOnce 订阅-只执行一次-同步
func (bus *EventBus) SubscribeOnce(observer interface{}) error {
	return bus.Register(observer, true, false, false)
}

// SubscribeOnceAsync 订阅-只执行一次-异步
func (bus *EventBus) SubscribeOnceAsync(observer interface{}) error {
	return bus.Register(observer, true, true, false)
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
func (bus *EventBus) Unsubscribe(observer interface{}) error {
	topic, t, fn, err := bus.checkObserver(observer)
	if err != nil {
		return err
	}
	for i := range topic[:] {
		function, ok := t.MethodByName(fn)
		if !ok {
			continue
		}
		bus.removeHandler(topic[i], bus.findHandlerIdx(topic[i], function.Func))
	}
	return nil
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

// Publish 推送
func (bus *EventBus) Publish(topic string, args ...interface{}) {
	if handlerInterface, ok := bus.handlers.Load(topic); ok {
		handlers := handlerInterface.([]*eventHandler)
		if len(handlers) == 0 {
			return
		}
		for i, handler := range handlers {
			if handler.flagOnce {
				bus.removeHandler(topic, i)
			}
			if !handler.async {
				bus.doPublish(handler, args...)
			} else {
				bus.wg.Add(1)
				if handler.transactional {
					handler.Lock()
				}
				go bus.doPublishAsync(handlers[i], args...)
			}
		}
	}
}

func (bus *EventBus) doPublish(handler *eventHandler, args ...interface{}) {
	passedArguments := bus.setUpPublish(handler, args...)
	handler.callBack.Call(passedArguments)
}

func (bus *EventBus) doPublishAsync(handler *eventHandler, args ...interface{}) {
	defer bus.wg.Done()
	if handler.transactional {
		defer handler.Unlock()
	}
	bus.doPublish(handler, args...)
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
	passedArguments := make([]reflect.Value, 0, len(args)+1)
	passedArguments = append(passedArguments, reflect.ValueOf(callback.observer))
	for i := range args[:] {
		if args[i] == nil {
			passedArguments = append(passedArguments, reflect.New(funcType.In(i)).Elem())
		} else {
			passedArguments = append(passedArguments, reflect.ValueOf(args[i]))
		}
	}

	return passedArguments
}

// WaitAsync 等待
func (bus *EventBus) WaitAsync() {
	bus.wg.Wait()
}

// WaitAsync 等待
func (bus *EventBus) Stop() {
	bus.handlers.Range(func(topic, value interface{}) bool {
		handlers := value.([]*eventHandler)
		for i := range handlers[:] {
			t, fn, ok := bus.checkStop(handlers[i].observer)
			if !ok {
				return false
			}
			function, ok := t.MethodByName(fn)
			if !ok {
				continue
			}
			function.Func.Call([]reflect.Value{reflect.ValueOf(handlers[i].observer)})
			_ = bus.Unsubscribe(handlers[i].observer)
		}

		return true
	})
}
func (bus *EventBus) checkStop(observer interface{}) (reflect.Type, string, bool) {
	var fn string
	var t reflect.Type
	t = reflect.TypeOf(observer)
	if t.Kind() != reflect.Struct {
		return t, fn, false
	}
	fnField, _ := t.FieldByName("Fn")
	if fnField.Tag == "" {
		return t, fn, false
	}
	fn, ok := fnField.Tag.Lookup("stop")
	if !ok || fn == "" {
		return t, fn, false
	}

	return t, fn, true
}
