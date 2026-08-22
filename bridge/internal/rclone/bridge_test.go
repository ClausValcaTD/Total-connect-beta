package rclone_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/totalconnect/bridge/internal/rclone"
)

func TestListRemotes(t *testing.T) {
	b := rclone.NewBridge()
	remotes, err := b.ListRemotes()
	if err != nil {
		t.Fatalf("ListRemotes failed: %v", err)
	}

	foundLocal := false
	for _, r := range remotes {
		if r.Name == "local" {
			foundLocal = true
			break
		}
	}
	if !foundLocal {
		t.Errorf("expected 'local' in remotes list, got: %+v", remotes)
	}
}

func TestListFilesLocal(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file and directory
	file1Path := filepath.Join(tempDir, "file1.txt")
	if err := os.WriteFile(file1Path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subDir: %v", err)
	}

	b := rclone.NewBridge()
	files, err := b.ListFiles("local", tempDir)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(files))
	}

	foundFile1 := false
	foundSubdir := false

	for _, f := range files {
		if filepath.Base(f.Path) == "file1.txt" {
			foundFile1 = true
			if f.IsDir {
				t.Errorf("expected file1.txt to not be a dir")
			}
			if f.Size != 11 {
				t.Errorf("expected file1.txt size 11, got %d", f.Size)
			}
		}
		if filepath.Base(f.Path) == "subdir" {
			foundSubdir = true
			if !f.IsDir {
				t.Errorf("expected subdir to be a dir")
			}
		}
	}

	if !foundFile1 {
		t.Errorf("file1.txt not found in ListFiles output")
	}
	if !foundSubdir {
		t.Errorf("subdir not found in ListFiles output")
	}
}

func TestTransferAndProgress(t *testing.T) {
	tempSrc := t.TempDir()
	tempDst := t.TempDir()

	srcFile := filepath.Join(tempSrc, "data.bin")
	content := []byte("sample binary data for testing transfer bridge")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to write src file: %v", err)
	}

	dstFile := filepath.Join(tempDst, "data.bin")

	b := rclone.NewBridge()
	jobID, err := b.Transfer(srcFile, dstFile, "upload")
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	if jobID == "" {
		t.Fatalf("expected non-empty jobID")
	}

	// Wait for background transfer job to complete
	var finalProg rclone.Progress
	for i := 0; i < 50; i++ {
		prog, pErr := b.GetProgress(jobID)
		if pErr != nil {
			t.Fatalf("GetProgress failed: %v", pErr)
		}
		if prog.Status == "completed" || prog.Status == "failed" {
			finalProg = prog
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalProg.Status != "completed" {
		t.Fatalf("expected job status 'completed', got '%s' (err: %s)", finalProg.Status, finalProg.Error)
	}

	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("transferred content mismatch. Expected '%s', got '%s'", string(content), string(dstContent))
	}
}

func TestGetProgressUnknownJob(t *testing.T) {
	b := rclone.NewBridge()
	_, err := b.GetProgress("unknown-job-id")
	if err == nil {
		t.Errorf("expected error for unknown job ID, got nil")
	}
}
