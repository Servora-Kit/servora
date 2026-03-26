package sse

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Event struct {
	ID    string
	Event string
	Data  string
}

func WriteEvent(w io.Writer, e Event) error {
	if e.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", e.ID); err != nil {
			return err
		}
	}
	if e.Event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", e.Event); err != nil {
			return err
		}
	}
	for _, line := range strings.Split(e.Data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func NewStaticHandler(e Event) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setHeaders(w)

		if err := WriteEvent(w, e); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func NewTickerHandler(interval time.Duration, maxEvents int) http.HandlerFunc {
	if interval <= 0 {
		interval = time.Second
	}

	return func(w http.ResponseWriter, r *http.Request) {
		setHeaders(w)
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		sent := 0
		for {
			select {
			case <-r.Context().Done():
				return
			case t := <-ticker.C:
				e := Event{Event: "tick", Data: t.UTC().Format(time.RFC3339Nano)}
				if err := WriteEvent(w, e); err != nil {
					return
				}
				flusher.Flush()
				sent++
				if maxEvents > 0 && sent >= maxEvents {
					return
				}
			}
		}
	}
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
