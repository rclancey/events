package events

import (
	"fmt"
	"time"
)

const (
	EventTypeHandlerAdded   = "listener-add"
	EventTypeHandlerRemoved = "listener-remove"
	EventTypeHandlerError   = "listener-error"
)

type Valuer interface {
	GetValue() float64
}

type Event interface {
	GetType() string
	GetTime() time.Time
	GetData() interface{}
	As(eventType string) Event
}

type ValueEvent interface {
	Event
	Valuer
}

type MessageEvent interface {
	Event
	GetMessage() string
}

type basicEvent struct {
	Type    string      `json:"type"`
	Time    time.Time   `json:"time"`
	Data    interface{} `json:"data,omitempty"`
}

func (ev *basicEvent) GetType() string {
	return ev.Type
}

func (ev *basicEvent) GetTime() time.Time {
	return ev.Time
}

func (ev *basicEvent) GetData() interface{} {
	return ev.Data
}

func (ev *basicEvent) As(eventType string) Event {
	return &basicEvent{
		Type: eventType,
		Time: ev.Time,
		Data: ev.Data,
	}
}

type valueEvent struct {
	Event
	Value float64 `json:"value"`
}

func (ev *valueEvent) GetValue() float64 {
	return ev.Value
}

func (ev *valueEvent) As(eventType string) Event {
	return &valueEvent{ev.Event.As(eventType), ev.Value}
}

type messageEvent struct {
	Event
	Message string      `json:"message"`
}

func (ev *messageEvent) GetMessage() string {
	return ev.Message
}

func (ev *messageEvent) As(eventType string) Event {
	return &messageEvent{ev.Event.As(eventType), ev.Message}
}

func NewEvent(evtType string, data interface{}) Event {
	base := &basicEvent{Type: evtType, Time: time.Now().In(time.UTC)}
	switch tdata := data.(type) {
	case float64:
		return &valueEvent{base, tdata}
	case float32:
		return &valueEvent{base, float64(tdata)}
	case int:
		return &valueEvent{base, float64(tdata)}
	case int64:
		return &valueEvent{base, float64(tdata)}
	case int32:
		return &valueEvent{base, float64(tdata)}
	case int16:
		return &valueEvent{base, float64(tdata)}
	case int8:
		return &valueEvent{base, float64(tdata)}
	case uint:
		return &valueEvent{base, float64(tdata)}
	case uint64:
		return &valueEvent{base, float64(tdata)}
	case uint32:
		return &valueEvent{base, float64(tdata)}
	case uint16:
		return &valueEvent{base, float64(tdata)}
	case uint8:
		return &valueEvent{base, float64(tdata)}
	case string:
		return &messageEvent{base, tdata}
	case map[string]interface{}:
		base.Data = tdata
		val, ok := tdata["value"]
		if ok {
			switch tval := val.(type) {
			case float64:
				return &valueEvent{base, tval}
			case float32:
				return &valueEvent{base, float64(tval)}
			case int:
				return &valueEvent{base, float64(tval)}
			case int64:
				return &valueEvent{base, float64(tval)}
			case int32:
				return &valueEvent{base, float64(tval)}
			case int16:
				return &valueEvent{base, float64(tval)}
			case int8:
				return &valueEvent{base, float64(tval)}
			case uint:
				return &valueEvent{base, float64(tval)}
			case uint64:
				return &valueEvent{base, float64(tval)}
			case uint32:
				return &valueEvent{base, float64(tval)}
			case uint16:
				return &valueEvent{base, float64(tval)}
			case uint8:
				return &valueEvent{base, float64(tval)}
			case Valuer:
				return &valueEvent{base, tval.GetValue()}
			case string:
				return &messageEvent{base, tval}
			case fmt.Stringer:
				return &messageEvent{base, tval.String()}
			}
		}
		msg, ok := tdata["message"]
		if ok {
			switch tmsg := msg.(type) {
			case string:
				return &messageEvent{base, tmsg}
			case fmt.Stringer:
				return &messageEvent{base, tmsg.String()}
			}
		}
		return base
	case Valuer:
		base.Data = data
		return &valueEvent{base, tdata.GetValue()}
	case fmt.Stringer:
		base.Data = data
		return &messageEvent{base, tdata.String()}
	}
	base.Data = data
	return base
}
