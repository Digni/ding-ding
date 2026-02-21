package logging

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	RequestIDHeader = "X-Request-Id"
)

var correlationCounter uint64

type PayloadMetadata struct {
	Transport     string
	ContentBytes  int64
	FieldNames    []string
	FieldCount    int
	PayloadShape  string
	ParseStatus   string
	ContainsBody  bool
	ContainsQuery bool
}

func NewOperationID() string {
	return newCorrelationID("op")
}

func NewRequestID() string {
	return newCorrelationID("req")
}

func EnsureRequestID(existing string) string {
	trimmed := strings.TrimSpace(existing)
	if trimmed != "" {
		return trimmed
	}

	return NewRequestID()
}

func PayloadMetadataFromBody(raw []byte, contentLength int64, transport string) PayloadMetadata {
	metadata := PayloadMetadata{
		Transport:    normalizeTransport(transport),
		ContentBytes: normalizeContentBytes(contentLength, int64(len(raw))),
		PayloadShape: "empty",
		ParseStatus:  "empty",
		ContainsBody: true,
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return metadata
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		metadata.PayloadShape = "malformed"
		metadata.ParseStatus = "malformed"
		return metadata
	}

	metadata.ParseStatus = "ok"
	metadata.PayloadShape = payloadShape(decoded)

	if object, ok := decoded.(map[string]any); ok {
		metadata.FieldNames = sortedKeys(object)
		metadata.FieldCount = len(metadata.FieldNames)
	}

	return metadata
}

func PayloadMetadataFromQuery(fieldNames []string, queryBytes int64) PayloadMetadata {
	normalized := append([]string(nil), fieldNames...)
	for i := range normalized {
		normalized[i] = strings.TrimSpace(normalized[i])
	}
	compact := normalized[:0]
	for _, name := range normalized {
		if name != "" {
			compact = append(compact, name)
		}
	}
	sort.Strings(compact)

	shape := "empty"
	if len(compact) > 0 {
		shape = "object"
	}

	return PayloadMetadata{
		Transport:     "query",
		ContentBytes:  normalizeContentBytes(queryBytes, queryBytes),
		FieldNames:    compact,
		FieldCount:    len(compact),
		PayloadShape:  shape,
		ParseStatus:   "n/a",
		ContainsQuery: true,
	}
}

func (p PayloadMetadata) Fields() []any {
	return []any{
		"payload_transport", p.Transport,
		"payload_content_bytes", p.ContentBytes,
		"payload_shape", p.PayloadShape,
		"payload_parse_status", p.ParseStatus,
		"payload_field_count", p.FieldCount,
		"payload_field_names", p.FieldNames,
		"payload_contains_body", p.ContainsBody,
		"payload_contains_query", p.ContainsQuery,
	}
}

func newCorrelationID(prefix string) string {
	counter := atomic.AddUint64(&correlationCounter, 1)
	ts := time.Now().UTC().UnixMilli()
	return fmt.Sprintf("%s-%s-%s", prefix, strconv.FormatInt(ts, 36), strconv.FormatUint(counter, 36))
}

func normalizeTransport(transport string) string {
	trimmed := strings.TrimSpace(strings.ToLower(transport))
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func normalizeContentBytes(provided int64, fallback int64) int64 {
	if provided >= 0 {
		return provided
	}
	if fallback >= 0 {
		return fallback
	}
	return 0
}

func payloadShape(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string, bool, float64, nil:
		return "scalar"
	default:
		return "unknown"
	}
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
