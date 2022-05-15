package logging

import (
	"bytes"
	"strconv"
	"time"
)

// sseMessage
type sseMessage struct {
	data  string // data 必填参数 其他选填
	id    string //
	event string //
	retry time.Duration
}

func sseWithData(data string) []byte {
	s := sseMessage{data: data}
	return s.ToBytes()
}

func (s *sseMessage) ToBytes() []byte {
	if s == nil {
		return nil
	}
	var buf bytes.Buffer
	if s.id != "" {
		buf.WriteString("id:")
		buf.WriteString(s.id + "\n")
	}
	if s.event != "" {
		buf.WriteString("event:")
		buf.WriteString(s.event + "\n")
	}
	if s.retry != 0 {
		buf.WriteString("retry:")
		buf.WriteString(strconv.FormatInt(int64(s.retry/time.Millisecond), 10) + "\n")
	}
	if s.data != "" {
		buf.WriteString("data:")
		buf.WriteString(s.data)
		buf.WriteString("\n\n")
	}
	return buf.Bytes()
}
func (s *sseMessage) ToString() string {
	if s == nil {
		return ""
	}
	return string(s.ToBytes())
}
