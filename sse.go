package logging

import (
	"fmt"
	"time"
)

// sseMessage
type sseMessage struct {
	data  string // data 必填参数 其他选填
	id    string //
	event string //
	retry time.Duration
}

func sseWithData(data string) string {
	s := sseMessage{data: data}
	return s.ToString()
}

func (s *sseMessage) ToString() string {
	if s == nil {
		return ""
	}
	var id, event, retry, data string
	if s.id != "" {
		id = fmt.Sprintf("id:%s\n", s.id)
	}
	if s.event != "" {
		event = fmt.Sprintf("event:%s\n", s.event)
	}
	if s.retry != 0 {
		retry = fmt.Sprintf("retry:%d\n", s.retry/time.Millisecond)
	}
	if s.data != "" {
		data = fmt.Sprintf("data:%s\n\n", s.data)
	}
	info := id + event + retry + data
	return info
}
