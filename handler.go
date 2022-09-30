package events

import (
	"errors"
	"math"
	"math/rand"
	"time"
)

var ErrIgnored = errors.New("ignored")
var ErrExpired = errors.New("expired")
var ErrIncompatibleEvent = errors.New("incompatible event")

type EventHandler interface {
	ID() int64
	Call(Event) error
	Expired() bool
	LastError() error
}

type HandlerFunc func(Event) error

type Direction string
const (
	DirectionNone       = Direction("")
	DirectionIncreasing = Direction("increasing")
	DirectionDecreasing = Direction("decreasing")
	DirectionSteady     = Direction("steady")
	DirectionReverse    = Direction("reverse")
)

type basicEventHandler struct {
	id int64
	handler func(Event) error
	lastErr error
}

func NewEventHandler(handler HandlerFunc) EventHandler {
	return &basicEventHandler{
		id: rand.Int63(),
		handler: handler,
	}
}

func HandlerReference(id int64) EventHandler {
	return &basicEventHandler{
		id: id,
		handler: func(Event) error {
			return errors.New("not a real handler")
		},
	}
}

func (eh *basicEventHandler) ID() int64 {
	return eh.id
}

func (eh *basicEventHandler) Expired() bool {
	return false
}

func (eh *basicEventHandler) Call(ev Event) error {
	if eh.Expired() {
		return nil
	}
	err := eh.handler(ev)
	eh.lastErr = err
	if err != nil {
		if errors.Is(err, ErrIgnored) {
			return nil
		}
		return err
	}
	return nil
}

func (eh *basicEventHandler) LastError() error {
	return eh.lastErr
}

type maxCallsHandler struct {
	EventHandler
	maxCalls int
	calls int
}

func WithMaxCalls(h EventHandler, maxCalls int) EventHandler {
	if maxCalls <= 0 {
		return h
	}
	return &maxCallsHandler{h, maxCalls, 0}
}

func (h *maxCallsHandler) Call(ev Event) error {
	if h.calls >= h.maxCalls {
		return ErrExpired
	}
	err := h.EventHandler.Call(ev)
	if err != nil {
		if errors.Is(err, ErrIgnored) {
			return nil
		}
		return err
	}
	h.calls += 1
	return nil
}

func (h *maxCallsHandler) Expired() bool {
	if h.calls >= h.maxCalls {
		return true
	}
	return h.EventHandler.Expired()
}

type timeoutHandler struct {
	EventHandler
	endTime time.Time
}

func WithTimeout(h EventHandler, ttl time.Duration) EventHandler {
	if ttl <= 0 {
		return h
	}
	return &timeoutHandler{h, time.Now().Add(ttl)}
}

func (h *timeoutHandler) Call(ev Event) error {
	if ev.GetTime().After(h.endTime) {
		return ErrExpired
	}
	return h.EventHandler.Call(ev)
}

func (h *timeoutHandler) Expired() bool {
	if time.Now().After(h.endTime) {
		return true
	}
	return h.EventHandler.Expired()
}

type directionHandler struct {
	EventHandler
	targetDirection Direction
	lastValue float64
	currentDirection Direction
}

func WithDirection(h EventHandler, direction Direction) EventHandler {
	return &directionHandler{h, direction, math.NaN(), DirectionNone}
}

func (h *directionHandler) Call(ev Event) error {
	valEv, ok := ev.(ValueEvent)
	if !ok {
		return ErrIncompatibleEvent
	}
	val := valEv.GetValue()
	if math.IsNaN(val) {
		return ErrIgnored
	}
	if math.IsNaN(h.lastValue) {
		h.lastValue = val
		return ErrIgnored
	}
	var dir Direction
	if val < h.lastValue {
		dir = DirectionDecreasing
	} else if val > h.lastValue {
		dir = DirectionIncreasing
	} else {
		dir = DirectionSteady
		if h.targetDirection == dir {
			return h.EventHandler.Call(ev)
		}
		return ErrIgnored
	}
	if h.targetDirection == DirectionReverse {
		if dir != h.currentDirection {
			return h.EventHandler.Call(ev)
		}
		return ErrIgnored
	}
	h.currentDirection = dir
	if dir == h.targetDirection {
		return h.EventHandler.Call(ev)
	}
	return ErrIgnored
}

type thresholdHandler struct {
	EventHandler
	direction Direction
	triggerVal float64
	resetVal float64
	triggered bool
}

func WithThreshold(h EventHandler, direction Direction, triggerVal, resetVal float64) EventHandler {
	return &thresholdHandler{h, direction, triggerVal, resetVal, false}
}

func (h *thresholdHandler) Call(ev Event) error {
	valEv, ok := ev.(ValueEvent)
	if !ok {
		return ErrIncompatibleEvent
	}
	val := valEv.GetValue()
	if math.IsNaN(val) {
		return ErrIgnored
	}
	if h.triggered {
		switch h.direction {
		case DirectionDecreasing:
			if val >= h.resetVal {
				h.triggered = false
			}
		case DirectionIncreasing:
			if val <= h.resetVal {
				h.triggered = false
			}
		}
		return ErrIgnored
	}
	switch h.direction {
	case DirectionDecreasing:
		if val <= h.triggerVal {
			h.triggered = true
			return h.EventHandler.Call(ev)
		}
	case DirectionIncreasing:
		if val >= h.triggerVal {
			h.triggered = true
			return h.EventHandler.Call(ev)
		}
	}
	return ErrIgnored
}

type rangeHandler struct {
	EventHandler
	min float64
	max float64
}

func WithRange(h EventHandler, min, max float64) EventHandler {
	return &rangeHandler{h, min, max}
}

func (h *rangeHandler) Call(ev Event) error {
	valEv, ok := ev.(ValueEvent)
	if !ok {
		return ErrIncompatibleEvent
	}
	val := valEv.GetValue()
	if math.IsNaN(val) {
		return ErrIgnored
	}
	if h.min > h.max {
		if val < h.min || val > h.max {
			return h.EventHandler.Call(ev)
		}
		return ErrIgnored
	}
	if val < h.min || val > h.max {
		return ErrIgnored
	}
	return h.EventHandler.Call(ev)
}

type debounceHandler struct {
	EventHandler
	ttl time.Duration
	last time.Time
}

func WithDebounce(h EventHandler, ttl time.Duration) EventHandler {
	if ttl <= 0 {
		return h
	}
	return &debounceHandler{h, ttl, time.Now().Add(-ttl)}
}

func (h *debounceHandler) Call(ev Event) error {
	t := ev.GetTime()
	if h.last.Add(h.ttl).After(t) {
		return ErrIgnored
	}
	h.last = t
	return h.EventHandler.Call(ev)
}
