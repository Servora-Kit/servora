package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestFileHandler_WritesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "app.log")
	h := buildFileHandler(nil, &corev1.Log_FileBackend{
		Path:    p,
		MaxSize: 1,
	}, slog.LevelInfo)
	if h == nil {
		t.Fatal("file handler must not be nil")
	}
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
	h := buildFileHandler(nil, nil, slog.LevelInfo)
	if h != nil {
		t.Error("nil config should return nil handler")
	}
}

func TestFileHandler_EmptyPath(t *testing.T) {
	h := buildFileHandler(nil, &corev1.Log_FileBackend{}, slog.LevelInfo)
	if h != nil {
		t.Error("empty path should return nil handler")
	}
}

func TestFileHandler_TextFormat(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "text.log")
	h := buildFileHandler(nil, &corev1.Log_FileBackend{
		Path:   p,
		Format: corev1.Log_LOG_FORMAT_TEXT,
	}, slog.LevelInfo)
	if h == nil {
		t.Fatal("text file handler must not be nil")
	}
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
