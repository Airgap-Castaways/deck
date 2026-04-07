package logs

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	cliFormatMu            sync.RWMutex
	defaultCLIFormat       = "text"
	defaultCLIColorEnabled bool
)

var reservedCLIAttrKeys = map[string]struct{}{
	"ts":        {},
	"level":     {},
	"component": {},
	"event":     {},
	"message":   {},
}

type CLIEvent struct {
	TS        time.Time
	Level     string
	Component string
	Event     string
	Message   string
	Attrs     map[string]any
}

func FormatCLIText(event CLIEvent) string {
	normalized := normalizeCLIEvent(event)
	tsKeyCode, tsValueCode := cliFieldCodes("ts", normalized.TS.Format(time.RFC3339Nano))
	levelKeyCode, levelValueCode := cliFieldCodes("level", normalized.Level)
	componentKeyCode, componentValueCode := cliFieldCodes("component", normalized.Component)
	eventKeyCode, eventValueCode := cliFieldCodes("event", normalized.Event)
	parts := []string{
		colorizeCLIField("ts", normalized.TS.Format(time.RFC3339Nano), tsKeyCode, tsValueCode),
		colorizeCLIField("level", normalized.Level, levelKeyCode, levelValueCode),
		colorizeCLIField("component", normalized.Component, componentKeyCode, componentValueCode),
		colorizeCLIField("event", normalized.Event, eventKeyCode, eventValueCode),
	}
	if normalized.Message != "" {
		messageKeyCode, messageValueCode := cliFieldCodes("message", normalized.Message)
		parts = append(parts, colorizeCLIField("message", normalized.Message, messageKeyCode, messageValueCode))
	}
	keys := make([]string, 0, len(normalized.Attrs))
	for key := range normalized.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		keyCode, valueCode := cliFieldCodes(key, normalized.Attrs[key])
		parts = append(parts, colorizeCLIField(key, normalized.Attrs[key], keyCode, valueCode))
	}
	return strings.Join(parts, " ")
}

func FormatCLIJSON(event CLIEvent) ([]byte, error) {
	normalized := normalizeCLIEvent(event)
	payload := map[string]any{
		"ts":        normalized.TS.Format(time.RFC3339Nano),
		"level":     normalized.Level,
		"component": normalized.Component,
		"event":     normalized.Event,
	}
	if normalized.Message != "" {
		payload["message"] = normalized.Message
	}
	for key, value := range normalized.Attrs {
		payload[key] = value
	}
	return json.Marshal(payload)
}

func SetCLIFormat(format string) {
	cliFormatMu.Lock()
	defer cliFormatMu.Unlock()
	defaultCLIFormat = normalizeCLIFormat(format)
}

func CLIFormat() string {
	cliFormatMu.RLock()
	defer cliFormatMu.RUnlock()
	return defaultCLIFormat
}

func RenderCLI(event CLIEvent, format string) (string, error) {
	switch normalizeCLIFormat(format) {
	case "json":
		raw, err := FormatCLIJSON(event)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	default:
		return FormatCLIText(event), nil
	}
}

func RenderDefaultCLI(event CLIEvent) (string, error) {
	return RenderCLI(event, CLIFormat())
}

func normalizeCLIFormat(format string) string {
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		return "json"
	}
	return "text"
}

func normalizeCLIEvent(event CLIEvent) CLIEvent {
	if event.TS.IsZero() {
		event.TS = time.Now().UTC()
	} else {
		event.TS = event.TS.UTC()
	}
	event.Level = strings.TrimSpace(strings.ToLower(event.Level))
	if event.Level == "" {
		event.Level = "info"
	}
	event.Component = strings.TrimSpace(event.Component)
	if event.Component == "" {
		event.Component = "cli"
	}
	event.Event = strings.TrimSpace(event.Event)
	if event.Event == "" {
		event.Event = "log"
	}
	event.Message = strings.TrimSpace(event.Message)
	event.Attrs = normalizeCLIAttrs(event.Attrs)
	if len(event.Attrs) == 0 {
		event.Attrs = nil
	}
	return event
}

func normalizeCLIAttrs(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	normalized := make(map[string]any, len(attrs))
	for key, value := range attrs {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || omitCLIAttrValue(value) {
			continue
		}
		if _, reserved := reservedCLIAttrKeys[trimmedKey]; reserved {
			continue
		}
		normalized[trimmedKey] = normalizeCLIAttrValue(value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func omitCLIAttrValue(value any) bool {
	if value == nil {
		return true
	}
	if err, ok := value.(error); ok {
		return strings.TrimSpace(err.Error()) == ""
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return true
	}
	if rv.Kind() == reflect.String {
		return strings.TrimSpace(rv.String()) == ""
	}
	return false
}

func normalizeCLIAttrValue(value any) any {
	if err, ok := value.(error); ok {
		return strings.TrimSpace(err.Error())
	}
	return value
}

func formatCLIValue(value any) string {
	switch v := value.(type) {
	case nil:
		return `""`
	case string:
		return quoteCLIString(v)
	case []byte:
		return quoteCLIString(string(v))
	case error:
		return quoteCLIString(v.Error())
	case time.Duration:
		return quoteCLIString(v.String())
	case time.Time:
		return quoteCLIString(v.UTC().Format(time.RFC3339Nano))
	case fmt.Stringer:
		return quoteCLIString(v.String())
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.String:
			return quoteCLIString(rv.String())
		case reflect.Bool:
			if rv.Bool() {
				return "true"
			}
			return "false"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.FormatInt(rv.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return strconv.FormatUint(rv.Uint(), 10)
		case reflect.Float32, reflect.Float64:
			return strconv.FormatFloat(rv.Float(), 'f', -1, 64)
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return quoteCLIString(fmt.Sprint(v))
		}
		return quoteCLIString(string(raw))
	}
}

func quoteCLIString(raw string) string {
	if raw == "" {
		return `""`
	}
	if strings.IndexFunc(raw, needsCLIQuoting) == -1 {
		return raw
	}
	return strconv.Quote(raw)
}

func needsCLIQuoting(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '"', '=':
		return true
	default:
		return false
	}
}
