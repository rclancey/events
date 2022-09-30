package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/rclancey/encoding-form"
)

func WebhookFunc(method, uri string, headers http.Header) HandlerFunc {
	h := headers.Clone()
	h.Set("Content-Type", "application/json")
	client := http.Client{}
	mutex := &sync.Mutex{}
	return func(ev Event) error {
		mutex.Lock()
		defer mutex.Unlock()
		var body io.Reader
		var bodySize int
		var u string
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch {
			u = uri
			data, err := json.Marshal(ev)
			if err != nil {
				return err
			}
			body = bytes.NewReader(data)
			bodySize = len(data)
		} else {
			xu, err := url.Parse(uri)
			if err != nil {
				return err
			}
			bodyData, err := form.MarshalForm(ev.GetData())
			if err != nil {
				return err
			}
			query, err := url.ParseQuery(string(bodyData))
			if err != nil {
				return err
			}
			for k, vals := range xu.Query() {
				query[k] = vals
			}
			xu.RawQuery = query.Encode()
			u = xu.String()
		}
		req, err := http.NewRequest(method, u, body)
		if err != nil {
			return err
		}
		req.Header = h.Clone()
		if body != nil {
			req.Header.Set("Content-Length", strconv.Itoa(bodySize))
		}
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 400 {
			return errors.New(res.Status)
		}
		return nil
	}
}

type Webhook struct {
	Method string `json:"method"`
	URL string `json:"url"`
	Headers http.Header `json:"header,omitempty"`
	Debounce *time.Duration `json:"debounce"`
	Direction *Direction `json:"direction,omitempty"`
	TriggerValue *float64 `json:"trigger_value,omitempty"`
	ResetValue *float64 `json:"reset_value,omitempty"`
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
	MaxCalls int `json:"max_calls,omitempty"`
	TTL time.Duration `json:"ttl,omitempty"`
}

func (hook *Webhook) Handler() EventHandler {
	h := NewEventHandler(WebhookFunc(hook.Method, hook.URL, hook.Headers))
	if hook.Debounce != nil {
		h = WithDebounce(h, *hook.Debounce)
	}
	if hook.Min != nil && hook.Max != nil {
		h = WithRange(h, *hook.Min, *hook.Max)
	}
	if hook.Direction != nil {
		if hook.TriggerValue != nil && hook.ResetValue != nil {
			h = WithThreshold(h, *hook.Direction, *hook.TriggerValue, *hook.ResetValue)
		} else {
			h = WithDirection(h, *hook.Direction)
		}
	}
	return WithTimeout(WithMaxCalls(h, hook.MaxCalls), hook.TTL)
}

func (hook *Webhook) Equals(other *Webhook) bool {
	if hook.Method != other.Method {
		return false
	}
	if hook.URL != other.URL {
		return false
	}
	return true
}
