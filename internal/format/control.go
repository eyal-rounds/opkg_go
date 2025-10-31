package format

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Paragraph represents a set of key/value pairs from a Debian control file.
type Paragraph struct {
	Fields map[string]string
}

// Value returns the value for the provided key, performing a case-insensitive
// lookup.
func (p Paragraph) Value(key string) string {
	for k, v := range p.Fields {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// ControlFile contains one or more paragraphs extracted from a Packages file
// or from the status database.
type ControlFile struct {
	Paragraphs []Paragraph
}

// ParseControl parses a Debian control formatted stream. The implementation is
// compatible with both Packages indexes and status files.
func ParseControl(r io.Reader) (*ControlFile, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var current Paragraph
	var file ControlFile

	flush := func() {
		if len(current.Fields) == 0 {
			return
		}
		file.Paragraphs = append(file.Paragraphs, current)
		current = Paragraph{Fields: map[string]string{}}
	}

	current = Paragraph{Fields: map[string]string{}}
	var lastKey string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			lastKey = ""
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if lastKey == "" {
				return nil, fmt.Errorf("continuation line encountered before key: %q", line)
			}
			current.Fields[lastKey] += "\n" + strings.TrimLeft(line, " \t")
			continue
		}

		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			return nil, fmt.Errorf("malformed control line: %q", line)
		}
		key := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])
		lastKey = key
		if current.Fields == nil {
			current.Fields = map[string]string{}
		}
		current.Fields[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()
	return &file, nil
}

// Keys returns the sorted list of keys present in the paragraph.
func (p Paragraph) Keys() []string {
	keys := make([]string, 0, len(p.Fields))
	for k := range p.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
