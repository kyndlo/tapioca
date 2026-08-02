package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	ProtocolVersion = 1
	MaxRequestBytes = 4 * 1024 * 1024
	MaxIDBytes      = 128
	MaxMethodBytes  = 128
	MaxJobIDBytes   = 128
)

type Request struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	JobID   string          `json:"job_id,omitempty"`
}

type Response struct {
	Version int            `json:"version"`
	Type    string         `json:"type"`
	ID      string         `json:"id"`
	Result  any            `json:"result,omitempty"`
	Error   *ProtocolError `json:"error,omitempty"`
}

type Event struct {
	Version   int    `json:"version"`
	Type      string `json:"type"`
	Event     string `json:"event"`
	JobID     string `json:"job_id"`
	Sequence  uint64 `json:"sequence"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data,omitempty"`
}

type EventName string

const (
	EventJobStarted   EventName = "job.started"
	EventJobProgress  EventName = "job.progress"
	EventJobLog       EventName = "job.log"
	EventJobCompleted EventName = "job.completed"
	EventJobFailed    EventName = "job.failed"
)

type ProtocolError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Details   any    `json:"details,omitempty"`
}

func (e *ProtocolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func DecodeRequest(line []byte) (Request, *ProtocolError) {
	var request Request
	if len(line) > MaxRequestBytes {
		return Request{}, invalidRequest("request exceeds the 4 MiB limit", nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return Request{}, invalidRequest("request must be valid JSON", err.Error())
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Request{}, invalidRequest("request must contain one JSON object", err.Error())
	}
	if request.Version == 0 {
		return request, invalidRequest("version is required", nil)
	}
	if request.Version != ProtocolVersion {
		return request, &ProtocolError{
			Code:      "unsupported_version",
			Message:   fmt.Sprintf("protocol version %d is not supported", request.Version),
			Retryable: false,
			Details:   map[string]any{"supported": []int{ProtocolVersion}},
		}
	}
	if request.Type != "request" {
		return request, invalidRequest(`type must be "request"`, nil)
	}
	if strings.TrimSpace(request.ID) == "" {
		return request, invalidRequest("id is required", nil)
	}
	if len(request.ID) > MaxIDBytes {
		return request, invalidRequest("id exceeds the 128-byte limit", nil)
	}
	if strings.TrimSpace(request.Method) == "" {
		return request, invalidRequest("method is required", nil)
	}
	if len(request.Method) > MaxMethodBytes {
		return request, invalidRequest("method exceeds the 128-byte limit", nil)
	}
	if len(request.JobID) > MaxJobIDBytes {
		return request, invalidRequest("job_id exceeds the 128-byte limit", nil)
	}
	if len(request.Params) > 0 && !json.Valid(request.Params) {
		return request, invalidRequest("params must be valid JSON", nil)
	}
	return request, nil
}

func Success(request Request, result any) Response {
	return Response{
		Version: ProtocolVersion,
		Type:    "response",
		ID:      request.ID,
		Result:  result,
	}
}

func Failure(request Request, protocolError *ProtocolError) Response {
	return Response{
		Version: ProtocolVersion,
		Type:    "response",
		ID:      request.ID,
		Error:   protocolError,
	}
}

func NewEvent(jobID string, name EventName, sequence uint64, timestamp time.Time, data any) Event {
	if sequence == 0 {
		sequence = 1
	}
	return Event{
		Version:   ProtocolVersion,
		Type:      "event",
		Event:     string(name),
		JobID:     jobID,
		Sequence:  sequence,
		Timestamp: timestamp.UTC().Format(time.RFC3339Nano),
		Data:      data,
	}
}

func invalidRequest(message string, details any) *ProtocolError {
	return &ProtocolError{
		Code:      "invalid_request",
		Message:   message,
		Retryable: false,
		Details:   details,
	}
}

func protocolError(err error) *ProtocolError {
	if err == nil {
		return nil
	}
	var typed *ProtocolError
	if errors.As(err, &typed) {
		return typed
	}
	return &ProtocolError{Code: "internal_error", Message: err.Error(), Retryable: true}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected trailing JSON value")
	}
	return err
}
