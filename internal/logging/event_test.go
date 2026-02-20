package logging

import (
	"reflect"
	"strings"
	"testing"
)

func TestEventEnsureRequestID(t *testing.T) {
	got := EnsureRequestID("  req-123  ")
	if got != "req-123" {
		t.Fatalf("EnsureRequestID preserved existing id = %q, want %q", got, "req-123")
	}

	generated := EnsureRequestID("   ")
	if !strings.HasPrefix(generated, "req-") {
		t.Fatalf("EnsureRequestID generated id = %q, want req- prefix", generated)
	}
}

func TestEventNewOperationID(t *testing.T) {
	first := NewOperationID()
	second := NewOperationID()

	if !strings.HasPrefix(first, "op-") {
		t.Fatalf("first operation id = %q, want op- prefix", first)
	}
	if !strings.HasPrefix(second, "op-") {
		t.Fatalf("second operation id = %q, want op- prefix", second)
	}
	if first == second {
		t.Fatalf("operation ids should be unique, got %q and %q", first, second)
	}
}

func TestEventPayloadMetadataFromBody_Empty(t *testing.T) {
	meta := PayloadMetadataFromBody(nil, 0, "json")

	if meta.Transport != "json" {
		t.Fatalf("transport = %q, want %q", meta.Transport, "json")
	}
	if meta.ContentBytes != 0 {
		t.Fatalf("content bytes = %d, want 0", meta.ContentBytes)
	}
	if meta.PayloadShape != "empty" {
		t.Fatalf("payload shape = %q, want %q", meta.PayloadShape, "empty")
	}
	if meta.ParseStatus != "empty" {
		t.Fatalf("parse status = %q, want %q", meta.ParseStatus, "empty")
	}
	if meta.FieldCount != 0 || len(meta.FieldNames) != 0 {
		t.Fatalf("expected no fields, got count=%d names=%v", meta.FieldCount, meta.FieldNames)
	}
}

func TestEventPayloadMetadataFromBody_Malformed(t *testing.T) {
	raw := []byte(`{"title":`)
	meta := PayloadMetadataFromBody(raw, int64(len(raw)), "json")

	if meta.PayloadShape != "malformed" {
		t.Fatalf("payload shape = %q, want %q", meta.PayloadShape, "malformed")
	}
	if meta.ParseStatus != "malformed" {
		t.Fatalf("parse status = %q, want %q", meta.ParseStatus, "malformed")
	}
	if meta.FieldCount != 0 || len(meta.FieldNames) != 0 {
		t.Fatalf("expected no field extraction for malformed payload, got count=%d names=%v", meta.FieldCount, meta.FieldNames)
	}
}

func TestEventPayloadMetadataFromBody_NormalObject(t *testing.T) {
	raw := []byte(`{"title":"build done","body":"sensitive content","agent":"claude"}`)
	meta := PayloadMetadataFromBody(raw, int64(len(raw)), "json")

	if meta.PayloadShape != "object" {
		t.Fatalf("payload shape = %q, want %q", meta.PayloadShape, "object")
	}
	if meta.ParseStatus != "ok" {
		t.Fatalf("parse status = %q, want %q", meta.ParseStatus, "ok")
	}
	if meta.FieldCount != 3 {
		t.Fatalf("field count = %d, want 3", meta.FieldCount)
	}

	wantFields := []string{"agent", "body", "title"}
	if !reflect.DeepEqual(meta.FieldNames, wantFields) {
		t.Fatalf("field names = %v, want %v", meta.FieldNames, wantFields)
	}

	for _, field := range meta.Fields() {
		if text, ok := field.(string); ok && strings.Contains(text, "sensitive content") {
			t.Fatalf("metadata fields leaked raw payload content: %v", meta.Fields())
		}
	}
}

func TestEventPayloadMetadataFromQuery(t *testing.T) {
	meta := PayloadMetadataFromQuery([]string{"agent", " message ", "", "title"}, 18)

	if meta.Transport != "query" {
		t.Fatalf("transport = %q, want %q", meta.Transport, "query")
	}
	if meta.ContentBytes != 18 {
		t.Fatalf("content bytes = %d, want 18", meta.ContentBytes)
	}
	if meta.PayloadShape != "object" {
		t.Fatalf("payload shape = %q, want %q", meta.PayloadShape, "object")
	}

	wantFields := []string{"agent", "message", "title"}
	if !reflect.DeepEqual(meta.FieldNames, wantFields) {
		t.Fatalf("field names = %v, want %v", meta.FieldNames, wantFields)
	}
	if meta.FieldCount != 3 {
		t.Fatalf("field count = %d, want 3", meta.FieldCount)
	}
}
