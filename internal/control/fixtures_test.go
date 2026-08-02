package control

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"
)

func TestGoldenFixturesConformToCanonicalSchema(t *testing.T) {
	schema := loadContractSchema(t)
	for _, name := range []string{"requests.ndjson", "responses.ndjson", "events.ndjson"} {
		forEachFixtureLine(t, name, func(t *testing.T, line []byte) {
			if err := validateEnvelope(schema, line); err != nil {
				t.Fatalf("fixture does not conform to envelope.schema.json: %v\n%s", err, line)
			}
		})
	}
}

func TestGoldenRequestFixturesDecode(t *testing.T) {
	forEachFixtureLine(t, "requests.ndjson", func(t *testing.T, line []byte) {
		request, err := DecodeRequest(line)
		if err != nil {
			t.Fatalf("DecodeRequest() error = %v for %s", err, line)
		}
		if request.Version != ProtocolVersion || request.ID == "" {
			t.Fatalf("request = %#v", request)
		}
	})
}

func TestCapabilitiesMatchCanonicalMethodAndEventEnums(t *testing.T) {
	schema := loadContractSchema(t)
	definitions := schema["$defs"].(map[string]any)
	request := definitions["request"].(map[string]any)
	requestProperties := request["properties"].(map[string]any)
	methodSchema := requestProperties["method"].(map[string]any)
	event := definitions["event"].(map[string]any)
	eventProperties := event["properties"].(map[string]any)
	eventSchema := eventProperties["event"].(map[string]any)

	wantMethods := stringsFromJSON(methodSchema["enum"].([]any))
	wantEvents := stringsFromJSON(eventSchema["enum"].([]any))
	capability := capabilities()
	gotMethods := append([]string(nil), capability["methods"].([]string)...)
	gotEvents := append([]string(nil), capability["events"].([]string)...)
	sort.Strings(wantMethods)
	sort.Strings(wantEvents)
	sort.Strings(gotMethods)
	sort.Strings(gotEvents)
	if fmt.Sprint(gotMethods) != fmt.Sprint(wantMethods) {
		t.Fatalf("capability methods = %v, schema methods = %v", gotMethods, wantMethods)
	}
	if fmt.Sprint(gotEvents) != fmt.Sprint(wantEvents) {
		t.Fatalf("capability events = %v, schema events = %v", gotEvents, wantEvents)
	}
}

func TestGoConstructorsConformToCanonicalSchema(t *testing.T) {
	schema := loadContractSchema(t)
	timestamp := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	envelopes := []any{
		Success(Request{ID: "req-success"}, map[string]any{"status": "ok"}),
		Failure(Request{ID: "req-error"}, &ProtocolError{
			Code: "internal_error", Message: "unavailable", Retryable: true,
		}),
		Failure(Request{}, invalidRequest("request must be valid JSON", "bad input")),
		NewEvent("job-1", EventJobStarted, 1, timestamp, map[string]any{
			"request_id": "req-success",
		}),
	}
	for index, envelope := range envelopes {
		encoded, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("json.Marshal(%d) error = %v", index, err)
		}
		if err := validateEnvelope(schema, encoded); err != nil {
			t.Fatalf("constructor envelope %d does not conform: %v\n%s", index, err, encoded)
		}
	}
}

func TestGoldenChatResultConformsToStrictChatSchema(t *testing.T) {
	schema := loadContractSchema(t)
	definitions := schema["$defs"].(map[string]any)
	chatSchema := definitions["chatResponse"].(map[string]any)
	found := false
	forEachFixtureLine(t, "responses.ndjson", func(t *testing.T, line []byte) {
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		if response["id"] != "req-chat-tools" {
			return
		}
		found = true
		if err := validateSchemaValue(schema, chatSchema, response["result"], "$.result"); err != nil {
			t.Fatalf("golden chat result violates chatResponse schema: %v", err)
		}
	})
	if !found {
		t.Fatal("responses.ndjson has no req-chat-tools fixture")
	}
}

func TestCanonicalSchemaRejectsInvalidEventNameAndSequence(t *testing.T) {
	schema := loadContractSchema(t)
	tests := []string{
		`{"version":1,"type":"event","event":"job.unknown","job_id":"job-1","sequence":1,"timestamp":"2026-08-01T12:00:00Z"}`,
		`{"version":1,"type":"event","event":"job.started","job_id":"job-1","sequence":0,"timestamp":"2026-08-01T12:00:00Z"}`,
	}
	for _, line := range tests {
		if err := validateEnvelope(schema, []byte(line)); err == nil {
			t.Fatalf("schema accepted invalid event: %s", line)
		}
	}
}

func TestUncorrelatedErrorIsOnlyValidEmptyResponseID(t *testing.T) {
	schema := loadContractSchema(t)
	valid := `{"version":1,"type":"response","id":"","error":{"code":"invalid_request","message":"bad JSON","retryable":false}}`
	if err := validateEnvelope(schema, []byte(valid)); err != nil {
		t.Fatalf("uncorrelated malformed error rejected: %v", err)
	}
	invalid := `{"version":1,"type":"response","id":"","error":{"code":"method_not_found","message":"unknown","retryable":false}}`
	if err := validateEnvelope(schema, []byte(invalid)); err == nil {
		t.Fatal("schema conformance helper accepted empty ID for a correlated error")
	}
}

func loadContractSchema(t *testing.T) map[string]any {
	t.Helper()
	path := filepath.Join("..", "..", "contracts", "control", "v1", "envelope.schema.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(contents, &schema); err != nil {
		t.Fatalf("schema is invalid JSON: %v", err)
	}
	return schema
}

func validateEnvelope(schema map[string]any, raw []byte) error {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	envelopeType, _ := envelope["type"].(string)
	definition := ""
	switch envelopeType {
	case "request":
		definition = "request"
	case "response":
		_, hasResult := envelope["result"]
		_, hasError := envelope["error"]
		if hasResult == hasError {
			return fmt.Errorf("response must contain exactly one of result or error")
		}
		if hasError {
			definition = "errorResponse"
			if envelope["id"] == "" {
				errorValue, _ := envelope["error"].(map[string]any)
				if errorValue["code"] != "invalid_request" {
					return fmt.Errorf("empty response id is reserved for invalid_request")
				}
			}
		} else {
			definition = "successResponse"
			if envelope["id"] == "" {
				return fmt.Errorf("successful response id must not be empty")
			}
		}
	case "event":
		definition = "event"
	default:
		return fmt.Errorf("unknown envelope type %q", envelopeType)
	}
	definitions, ok := schema["$defs"].(map[string]any)
	if !ok {
		return fmt.Errorf("schema has no $defs")
	}
	definitionSchema, ok := definitions[definition].(map[string]any)
	if !ok {
		return fmt.Errorf("schema has no %s definition", definition)
	}
	return validateSchemaValue(schema, definitionSchema, envelope, "$")
}

func validateSchemaValue(root, schema map[string]any, value any, path string) error {
	if reference, ok := schema["$ref"].(string); ok {
		const prefix = "#/$defs/"
		if len(reference) <= len(prefix) || reference[:len(prefix)] != prefix {
			return fmt.Errorf("%s: unsupported reference %q", path, reference)
		}
		definitions := root["$defs"].(map[string]any)
		referenced, ok := definitions[reference[len(prefix):]].(map[string]any)
		if !ok {
			return fmt.Errorf("%s: missing reference %q", path, reference)
		}
		return validateSchemaValue(root, referenced, value, path)
	}
	if constant, exists := schema["const"]; exists && !jsonValuesEqual(value, constant) {
		return fmt.Errorf("%s: got %v, want constant %v", path, value, constant)
	}
	if enum, ok := schema["enum"].([]any); ok {
		matched := false
		for _, candidate := range enum {
			if jsonValuesEqual(value, candidate) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s: %v is not in enum", path, value)
		}
	}
	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		properties, _ := schema["properties"].(map[string]any)
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			for key := range object {
				if _, known := properties[key]; !known {
					return fmt.Errorf("%s: unknown property %q", path, key)
				}
			}
		}
		if required, ok := schema["required"].([]any); ok {
			for _, item := range required {
				key := item.(string)
				if _, exists := object[key]; !exists {
					return fmt.Errorf("%s: required property %q is missing", path, key)
				}
			}
		}
		for key, item := range object {
			propertySchema, ok := properties[key].(map[string]any)
			if !ok {
				continue
			}
			if err := validateSchemaValue(root, propertySchema, item, path+"."+key); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: expected string", path)
		}
		if minimum, ok := schema["minLength"].(float64); ok && len(text) < int(minimum) {
			return fmt.Errorf("%s: shorter than minLength", path)
		}
		if maximum, ok := schema["maxLength"].(float64); ok && len(text) > int(maximum) {
			return fmt.Errorf("%s: longer than maxLength", path)
		}
		if schema["format"] == "date-time" {
			if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
				return fmt.Errorf("%s: invalid date-time: %w", path, err)
			}
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || number != math.Trunc(number) {
			return fmt.Errorf("%s: expected integer", path)
		}
		if minimum, ok := schema["minimum"].(float64); ok && number < minimum {
			return fmt.Errorf("%s: below minimum", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", path)
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		if maximum, ok := schema["maxItems"].(float64); ok && len(array) > int(maximum) {
			return fmt.Errorf("%s: exceeds maxItems", path)
		}
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for index, item := range array {
				if err := validateSchemaValue(root, itemSchema, item, path+"["+strconv.Itoa(index)+"]"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func jsonValuesEqual(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

func stringsFromJSON(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.(string))
	}
	return result
}

func forEachFixtureLine(t *testing.T, name string, check func(*testing.T, []byte)) {
	t.Helper()
	path := filepath.Join("..", "..", "contracts", "control", "v1", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		t.Run(name+"-line-"+strconv.Itoa(line), func(t *testing.T) {
			check(t, append([]byte(nil), scanner.Bytes()...))
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
}
