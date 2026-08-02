package control

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeRequest(t *testing.T) {
	request, err := DecodeRequest([]byte(
		`{"version":1,"type":"request","id":"req-1","method":"health.get","params":{},"job_id":"job-1"}`,
	))
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}
	if request.ID != "req-1" || request.JobID != "job-1" || request.Method != "health.get" {
		t.Fatalf("DecodeRequest() = %#v", request)
	}
}

func TestDecodeRequestEnforcesFourMiBLineBound(t *testing.T) {
	line := bytes.Repeat([]byte(" "), MaxRequestBytes+1)
	_, err := DecodeRequest(line)
	if err == nil || err.Code != "invalid_request" {
		t.Fatalf("oversized request error = %#v", err)
	}
}

func TestDecodeRequestRejectsMalformedAndInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		line string
		code string
	}{
		{name: "malformed json", line: `{`, code: "invalid_request"},
		{name: "unsupported version", line: `{"version":2,"type":"request","id":"1","method":"health.get"}`, code: "unsupported_version"},
		{name: "wrong envelope", line: `{"version":1,"type":"event","id":"1","method":"health.get"}`, code: "invalid_request"},
		{name: "missing id", line: `{"version":1,"type":"request","method":"health.get"}`, code: "invalid_request"},
		{name: "missing method", line: `{"version":1,"type":"request","id":"1"}`, code: "invalid_request"},
		{name: "unknown field", line: `{"version":1,"type":"request","id":"1","method":"health.get","extra":true}`, code: "invalid_request"},
		{name: "trailing object", line: `{"version":1,"type":"request","id":"1","method":"health.get"} {}`, code: "invalid_request"},
		{name: "long id", line: `{"version":1,"type":"request","id":"` + strings.Repeat("x", MaxIDBytes+1) + `","method":"health.get"}`, code: "invalid_request"},
		{name: "long method", line: `{"version":1,"type":"request","id":"1","method":"` + strings.Repeat("x", MaxMethodBytes+1) + `"}`, code: "invalid_request"},
		{name: "long job id", line: `{"version":1,"type":"request","id":"1","method":"health.get","job_id":"` + strings.Repeat("x", MaxJobIDBytes+1) + `"}`, code: "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRequest([]byte(test.line))
			if err == nil || err.Code != test.code {
				t.Fatalf("DecodeRequest() error = %#v, want code %q", err, test.code)
			}
		})
	}
}

func TestEventEnvelope(t *testing.T) {
	timestamp := time.Date(2026, time.August, 1, 12, 30, 0, 123, time.UTC)
	event := NewEvent("job-7", EventJobStarted, 3, timestamp, map[string]any{"percent": 42})
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if envelope["version"] != float64(1) || envelope["type"] != "event" ||
		envelope["job_id"] != "job-7" || envelope["event"] != "job.started" ||
		envelope["sequence"] != float64(3) ||
		envelope["timestamp"] != "2026-08-01T12:30:00.000000123Z" {
		t.Fatalf("event envelope = %#v", envelope)
	}
}

func TestEventSequenceStartsAtOne(t *testing.T) {
	event := NewEvent("job-1", EventJobStarted, 0, time.Now(), nil)
	if event.Sequence != 1 {
		t.Fatalf("sequence = %d, want 1", event.Sequence)
	}
}

func TestErrorEnvelopeAlwaysIncludesRetryable(t *testing.T) {
	encoded, err := json.Marshal(Failure(Request{ID: "req-1"}, invalidRequest("bad", nil)))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"retryable":false`) {
		t.Fatalf("encoded error does not contain retryable: %s", encoded)
	}
}
