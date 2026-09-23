package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestFileHandler_WritesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	h, closer := buildFileHandler(nil, &corev1.Log_FileBackend{
		Path:    proto.String(p),
		MaxSize: proto.Int32(1),
	}, slog.LevelInfo)
	if h == nil {
		t.Fatal("file handler must not be nil")
	}
	if closer == nil {
		t.Fatal("file handler closer must not be nil")
	}
	// Release the log file before t.TempDir cleanup removes the directory.
	t.Cleanup(func() { _ = closer(context.Background()) })
	slog.New(h).Info("file-test-line", "k", "v")

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(b), "file-test-line") {
		t.Errorf("log file missing expected content, got: %s", string(b))
	}
}

func TestFileHandler_NilConfig(t *testing.T) {
	h, closer := buildFileHandler(nil, nil, slog.LevelInfo)
	if h != nil {
		t.Error("nil config should return nil handler")
	}
	if closer != nil {
		t.Error("nil config should return nil closer")
	}
}

func TestFileHandler_TextFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "text.log")
	h, closer := buildFileHandler(nil, &corev1.Log_FileBackend{
		Path:   proto.String(p),
		Format: corev1.Log_LOG_FORMAT_TEXT,
	}, slog.LevelInfo)
	if h == nil {
		t.Fatal("text file handler must not be nil")
	}
	if closer == nil {
		t.Fatal("text file handler closer must not be nil")
	}
	// Release the log file before t.TempDir cleanup removes the directory.
	t.Cleanup(func() { _ = closer(context.Background()) })
	slog.New(h).Info("text-line")

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, "text-line") {
		t.Errorf("expected 'text-line', got: %s", content)
	}
}
