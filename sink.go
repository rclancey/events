package events

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclancey/generic"
)

type ListenerMeta struct {
	EventType string `json:"event_type"`
	HandlerID int64 `json:"handler_id"`
	Error string `json:"error,omitempty"`
}

type EventSink interface {
	AddEventListener(eventType string, handler EventHandler)
	RemoveEventListener(eventType string, handler EventHandler)
	Once(eventType string, handler EventHandler)
	Fire(ev Event)
	Emit(eventType string, data interface{})
	Log() []Event
	RegisterEventType(ev Event)
	ListEventTypes() []Event
}

type basicEventSink struct {
	listeners map[string][]EventHandler
	eventTypes map[string]Event
	mutex *sync.Mutex
	log *generic.LinkedList[Event]
	logTTL time.Duration
}

func NewEventSink(logTTL time.Duration) EventSink {
	return &basicEventSink{
		listeners: map[string][]EventHandler{},
		eventTypes: map[string]Event{},
		mutex: &sync.Mutex{},
		log: generic.NewLinkedList[Event](),
		logTTL: logTTL,
	}
}

func (es *basicEventSink) AddEventListener(eventType string, handler EventHandler) {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	es.listeners[eventType] = append(es.listeners[eventType], handler)
	go func() {
		data := &ListenerMeta{
			EventType: eventType,
			HandlerID: handler.ID(),
		}
		es.Emit(EventTypeHandlerAdded, data)
	}()
}

func (es *basicEventSink) RemoveEventListener(eventType string, handler EventHandler) {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	out := make([]EventHandler, 0, len(es.listeners[eventType]))
	id := handler.ID()
	evts := make([]Event, 0, 1)
	for _, eh := range es.listeners[eventType] {
		if eh.ID() != id {
			out = append(out, eh)
		} else {
			data := &ListenerMeta{
				EventType: eventType,
				HandlerID: id,
			}
			evts = append(evts, NewEvent(EventTypeHandlerRemoved, data))
		}
	}
	if len(out) == 0 {
		delete(es.listeners, eventType)
	} else {
		es.listeners[eventType] = out
	}
	if len(evts) > 0 {
		for _, ev := range evts {
			xev := ev
			go func() {
				es.Fire(xev)
			}()
		}
	}
}

func (es *basicEventSink) Once(eventType string, handler EventHandler) {
	es.AddEventListener(eventType, WithMaxCalls(handler, 1))
}

func (es *basicEventSink) Fire(ev Event) {
	eventType := ev.GetType()
	es.log.Unshift(ev)
	oldest := time.Now().Add(-es.logTTL)
	es.log.PopIf(func(ev Event) bool { return ev.GetTime().Before(oldest) })
	es.mutex.Lock()
	listeners := es.listeners[eventType]
	if _, ok := es.eventTypes[eventType]; !ok {
		es.eventTypes[eventType] = ev
	}
	es.mutex.Unlock()
	if len(listeners) == 0 {
		return
	}
	for _, h := range listeners {
		xh := h
		go func() {
			err := xh.Call(ev)
			if err != nil {
				if errors.Is(err, ErrExpired) {
					es.RemoveEventListener(eventType, xh)
					return
				}
				if !errors.Is(err, ErrIgnored) {
					data := &ListenerMeta{
						EventType: eventType,
						HandlerID: xh.ID(),
						Error: err.Error(),
					}
					es.Emit(EventTypeHandlerError, data)
				}
			}
			if xh.Expired() {
				es.RemoveEventListener(eventType, xh)
			}
		}()
	}
}

func (es *basicEventSink) Emit(eventType string, data interface{}) {
	ev := NewEvent(eventType, data)
	es.Fire(ev)
}

func (es *basicEventSink) Log() []Event {
	return es.log.Slice()
}

func (es *basicEventSink) RegisterEventType(ev Event) {
	es.mutex.Lock()
	es.eventTypes[ev.GetType()] = ev
	es.mutex.Unlock()
}

func (es *basicEventSink) ListEventTypes() []Event {
	es.mutex.Lock()
	evs := make([]Event, len(es.eventTypes))
	i := 0
	for _, v := range es.eventTypes {
		evs[i] = v
		i += 1
	}
	es.mutex.Unlock()
	sort.Slice(evs, func(i, j int) bool { return evs[i].GetType() < evs[j].GetType() })
	return evs
}

type PrefixedEventSource struct {
	EventSink
	prefix string
}

func NewPrefixedEventSource(prefix string, sink EventSink) EventSink {
	return &PrefixedEventSource{sink, prefix+"-"}
}

func (es *PrefixedEventSource) AddEventListener(eventType string, handler EventHandler) {
	es.EventSink.AddEventListener(es.prefix+eventType, handler)
}

func (es *PrefixedEventSource) RemoveEventListener(eventType string, handler EventHandler) {
	es.RemoveEventListener(es.prefix+eventType, handler)
}

func (es *PrefixedEventSource) Once(eventType string, handler EventHandler) {
	es.Once(es.prefix+eventType, handler)
}

func (es *PrefixedEventSource) As(ev Event) Event {
	return ev.As(es.prefix+ev.GetType())
}

func (es *PrefixedEventSource) Fire(ev Event) {
	es.EventSink.Fire(es.As(ev))
}

func (es *PrefixedEventSource) Emit(eventType string, data interface{}) {
	es.EventSink.Emit(es.prefix+eventType, data)
}

func (es *PrefixedEventSource) Filter(all []Event) []Event {
	filtered := make([]Event, 0, len(all))
	for _, ev := range all {
		if strings.HasPrefix(ev.GetType(), es.prefix) {
			filtered = append(filtered, ev.As(strings.TrimPrefix(ev.GetType(), es.prefix)))
		}
	}
	return filtered
}

func (es *PrefixedEventSource) Log() []Event {
	return es.Filter(es.EventSink.Log())
}

func (es *PrefixedEventSource) RegisterEventType(ev Event) {
	es.EventSink.RegisterEventType(es.As(ev))
}

func (es *PrefixedEventSource) ListEventTypes() []Event {
	return es.Filter(es.EventSink.ListEventTypes())
}
