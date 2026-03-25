package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Upsert sets key=value in the given .env file, preserving existing content.
// Creates the file if it does not exist.
func Upsert(path, key, value string) error {
	entries, err := read(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	found := false
	for i, e := range entries {
		if e.key == key {
			entries[i].value = value
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry{key: key, value: value})
	}

	return write(path, entries)
}

type entry struct {
	key   string
	value string
	raw   string // non-KV lines preserved as-is
}

func read(path string) ([]entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if k, v, ok := parseLine(line); ok {
			entries = append(entries, entry{key: k, value: v})
		} else {
			entries = append(entries, entry{raw: line})
		}
	}
	return entries, scanner.Err()
}

func parseLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	idx := strings.IndexByte(trimmed, '=')
	if idx <= 0 {
		return "", "", false
	}
	return trimmed[:idx], trimmed[idx+1:], true
}

func write(path string, entries []entry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for _, e := range entries {
		if e.key != "" {
			_, _ = fmt.Fprintf(w, "%s=%s\n", e.key, e.value)
		} else {
			_, _ = fmt.Fprintln(w, e.raw)
		}
	}
	return w.Flush()
}
