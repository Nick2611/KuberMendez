package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamFileTailReturnsLastLines(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "daemon.log")
	content := "one\ntwo\nthree\nfour\nfive\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	var buffer bytes.Buffer
	if err := StreamFileTail(&buffer, filePath, 3); err != nil {
		t.Fatalf("stream tail: %v", err)
	}

	if got, want := buffer.String(), "three\nfour\nfive\n"; got != want {
		t.Fatalf("tail = %q, want %q", got, want)
	}
}

func TestStreamFileTailReturnsWholeFileWhenShorterThanRequested(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "daemon.log")
	content := "one\ntwo\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	var buffer bytes.Buffer
	if err := StreamFileTail(&buffer, filePath, 50); err != nil {
		t.Fatalf("stream tail: %v", err)
	}

	if got := buffer.String(); got != content {
		t.Fatalf("tail = %q, want %q", got, content)
	}
}

func TestStreamFileTailWithNoTrailingNewline(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "daemon.log")
	content := "one\ntwo\nthree"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	var buffer bytes.Buffer
	if err := StreamFileTail(&buffer, filePath, 2); err != nil {
		t.Fatalf("stream tail: %v", err)
	}

	if got, want := buffer.String(), "two\nthree"; got != want {
		t.Fatalf("tail = %q, want %q", got, want)
	}
}
