package structuredpath

import (
	"fmt"
	"strconv"
	"strings"
)

type Segment struct {
	Key     string
	Index   int
	IsIndex bool
}

func Parse(raw string) ([]Segment, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("path is required")
	}
	if strings.HasPrefix(raw, "/") {
		return ParsePointer(raw)
	}
	return ParseAlias(raw)
}

func ParsePointer(raw string) ([]Segment, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("path is required")
	}
	if raw == "/" {
		return nil, nil
	}
	if !strings.HasPrefix(raw, "/") {
		return nil, fmt.Errorf("json pointer must start with /")
	}
	parts := strings.Split(strings.TrimPrefix(raw, "/"), "/")
	segments := make([]Segment, 0, len(parts))
	for _, part := range parts {
		unescaped := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		seg, err := segmentFromToken(unescaped)
		if err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}
	return segments, nil
}

func ParseAlias(raw string) ([]Segment, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("path is required")
	}
	tokens := make([]string, 0)
	var current strings.Builder
	inBracket := false
	inQuotes := false
	escaped := false
	justClosedBracket := false
	flush := func() error {
		segment := strings.TrimSpace(current.String())
		current.Reset()
		if segment == "" {
			return fmt.Errorf("invalid path %q", raw)
		}
		tokens = append(tokens, segment)
		return nil
	}
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch {
		case escaped:
			current.WriteByte(ch)
			escaped = false
			justClosedBracket = false
		case ch == '\\' && inQuotes:
			escaped = true
		case inQuotes:
			if ch == '"' {
				inQuotes = false
				continue
			}
			current.WriteByte(ch)
			justClosedBracket = false
		case inBracket:
			switch ch {
			case '"':
				if current.Len() == 0 {
					inQuotes = true
					continue
				}
				current.WriteByte(ch)
			case ']':
				if err := flush(); err != nil {
					return nil, err
				}
				inBracket = false
				justClosedBracket = true
			case ' ':
				if current.Len() == 0 {
					continue
				}
				current.WriteByte(ch)
				justClosedBracket = false
			default:
				current.WriteByte(ch)
				justClosedBracket = false
			}
		default:
			switch ch {
			case '"':
				inQuotes = true
				justClosedBracket = false
			case '.':
				if current.Len() == 0 {
					if justClosedBracket {
						justClosedBracket = false
						continue
					}
					return nil, fmt.Errorf("invalid path %q", raw)
				}
				if err := flush(); err != nil {
					return nil, err
				}
				justClosedBracket = false
			case '[':
				if current.Len() > 0 {
					if err := flush(); err != nil {
						return nil, err
					}
				}
				inBracket = true
				justClosedBracket = false
			case ' ':
				if current.Len() == 0 {
					continue
				}
				current.WriteByte(ch)
				justClosedBracket = false
			default:
				current.WriteByte(ch)
				justClosedBracket = false
			}
		}
	}
	if inBracket || inQuotes || escaped {
		return nil, fmt.Errorf("invalid path %q", raw)
	}
	if current.Len() > 0 {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	segments := make([]Segment, 0, len(tokens))
	for _, token := range tokens {
		seg, err := segmentFromToken(token)
		if err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}
	return segments, nil
}

func Canonicalize(raw string) (string, error) {
	segments, err := Parse(raw)
	if err != nil {
		return "", err
	}
	return ToPointer(segments), nil
}

func ToPointer(segments []Segment) string {
	if len(segments) == 0 {
		return "/"
	}
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment.IsIndex {
			parts = append(parts, strconv.Itoa(segment.Index))
			continue
		}
		parts = append(parts, strings.ReplaceAll(strings.ReplaceAll(segment.Key, "~", "~0"), "/", "~1"))
	}
	return "/" + strings.Join(parts, "/")
}

func segmentFromToken(token string) (Segment, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Segment{}, fmt.Errorf("path segment is empty")
	}
	if idx, err := strconv.Atoi(token); err == nil {
		if idx < 0 {
			return Segment{}, fmt.Errorf("invalid negative index %q", token)
		}
		return Segment{Index: idx, IsIndex: true}, nil
	}
	return Segment{Key: token}, nil
}
