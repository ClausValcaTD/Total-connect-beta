package rclone

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
)

// Remote represents a configured or available storage remote.
type Remote struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FileInfo represents metadata for a file or directory.
type FileInfo struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
}

// Progress tracks the status of an ongoing or completed transfer.
type Progress struct {
	JobID            string  `json:"job_id"`
	BytesTransferred int64   `json:"bytes_transferred"`
	TotalBytes       int64   `json:"total_bytes"`
	Percentage       float64 `json:"percentage"`
	Status           string  `json:"status"` // "queued", "in_progress", "completed", "failed"
	Error            string  `json:"error,omitempty"`
}

// Bridge wraps rclone functionality to connect to cloud backends and perform file operations.
type Bridge struct {
	mu       sync.RWMutex
	jobs     map[string]*Progress
	jobCounter int64
}

// NewBridge returns a new instance of Bridge.
func NewBridge() *Bridge {
	return &Bridge{
		jobs: make(map[string]*Progress),
	}
}

// ListRemotes returns a list of configured rclone remotes along with local filesystem.
func (b *Bridge) ListRemotes() ([]Remote, error) {
	sections := config.Data().GetSectionList()
	remotes := make([]Remote, 0, len(sections)+1)

	// Add local remote
	remotes = append(remotes, Remote{
		Name: "local",
		Type: "local",
	})

	for _, name := range sections {
		if name == "local" {
			continue
		}
		remoteType, _ := config.Data().GetValue(name, "type")
		remotes = append(remotes, Remote{
			Name: name,
			Type: remoteType,
		})
	}

	return remotes, nil
}

// ListFiles lists files and directories at the given path for a remote.
func (b *Bridge) ListFiles(remote string, dirPath string) ([]FileInfo, error) {
	ctx := context.Background()

	var target string
	if remote == "" || remote == "local" {
		if dirPath == "" {
			dirPath = "."
		}
		absPath, err := filepath.Abs(dirPath)
		if err != nil {
			absPath = dirPath
		}
		target = absPath
	} else {
		if strings.HasSuffix(remote, ":") {
			target = remote + dirPath
		} else if strings.Contains(remote, ":") {
			target = remote
		} else {
			target = remote + ":" + dirPath
		}
	}

	f, err := fs.NewFs(ctx, target)
	if err != nil {
		// Fallback for local directory if fs.NewFs has issue
		if remote == "" || remote == "local" {
			return b.listLocalFallback(dirPath)
		}
		return nil, fmt.Errorf("failed to initialize fs for %s: %w", target, err)
	}

	entries, err := f.List(ctx, "")
	if err != nil {
		if remote == "" || remote == "local" {
			return b.listLocalFallback(dirPath)
		}
		return nil, fmt.Errorf("failed to list entries for %s: %w", target, err)
	}

	var results []FileInfo
	for _, entry := range entries {
		var isDir bool
		var size int64
		var modTime time.Time

		switch e := entry.(type) {
		case fs.Object:
			isDir = false
			size = e.Size()
			modTime = e.ModTime(ctx)
		case fs.Directory:
			isDir = true
			size = e.Size()
			modTime = e.ModTime(ctx)
		default:
			size = entry.Size()
			modTime = entry.ModTime(ctx)
			isDir = entry.Size() < 0
		}

		results = append(results, FileInfo{
			Path:    entry.Remote(),
			Size:    size,
			IsDir:   isDir,
			ModTime: modTime,
		})
	}

	return results, nil
}

func (b *Bridge) listLocalFallback(dirPath string) ([]FileInfo, error) {
	if dirPath == "" {
		dirPath = "."
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var results []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		var size int64
		var modTime time.Time
		if err == nil {
			size = info.Size()
			modTime = info.ModTime()
		}
		results = append(results, FileInfo{
			Path:    entry.Name(),
			Size:    size,
			IsDir:   entry.IsDir(),
			ModTime: modTime,
		})
	}
	return results, nil
}

// Transfer initiates a file/directory transfer job and returns a job ID.
func (b *Bridge) Transfer(src string, dst string, direction string) (string, error) {
	id := atomic.AddInt64(&b.jobCounter, 1)
	jobID := fmt.Sprintf("job-%d", id)

	prog := &Progress{
		JobID:            jobID,
		BytesTransferred: 0,
		TotalBytes:       0,
		Percentage:       0.0,
		Status:           "in_progress",
	}

	b.mu.Lock()
	b.jobs[jobID] = prog
	b.mu.Unlock()

	go b.executeTransfer(jobID, src, dst, direction)

	return jobID, nil
}

func (b *Bridge) executeTransfer(jobID, src, dst, direction string) {
	ctx := context.Background()

	updateProgress := func(bytesTransferred, totalBytes int64, status, errStr string) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if p, ok := b.jobs[jobID]; ok {
			p.BytesTransferred = bytesTransferred
			p.TotalBytes = totalBytes
			if totalBytes > 0 {
				p.Percentage = (float64(bytesTransferred) / float64(totalBytes)) * 100.0
			} else if status == "completed" {
				p.Percentage = 100.0
			}
			p.Status = status
			p.Error = errStr
		}
	}

	// Try using rclone NewFs for transfer
	srcFs, srcErr := fs.NewFs(ctx, src)
	dstFs, dstErr := fs.NewFs(ctx, dst)

	if srcErr == nil && dstErr == nil {
		// Calculate total size if possible
		var totalSize int64
		if entries, err := srcFs.List(ctx, ""); err == nil {
			for _, entry := range entries {
				if obj, ok := entry.(fs.Object); ok {
					totalSize += obj.Size()
				}
			}
		}
		updateProgress(0, totalSize, "in_progress", "")

		err := operations.CopyFile(ctx, dstFs, srcFs, filepath.Base(dst), filepath.Base(src))
		if err == nil {
			updateProgress(totalSize, totalSize, "completed", "")
			return
		}
	}

	// Fallback for local files/directories transfer
	info, err := os.Stat(src)
	if err != nil {
		updateProgress(0, 0, "failed", err.Error())
		return
	}

	if !info.IsDir() {
		totalSize := info.Size()
		updateProgress(0, totalSize, "in_progress", "")

		if err := copyLocalFile(src, dst, func(copied int64) {
			updateProgress(copied, totalSize, "in_progress", "")
		}); err != nil {
			updateProgress(0, totalSize, "failed", err.Error())
			return
		}
		updateProgress(totalSize, totalSize, "completed", "")
	} else {
		// Directory copy fallback
		err := copyLocalDir(src, dst, func(copied, total int64) {
			updateProgress(copied, total, "in_progress", "")
		})
		if err != nil {
			updateProgress(0, 0, "failed", err.Error())
			return
		}
		updateProgress(100, 100, "completed", "")
	}
}

func copyLocalFile(src, dst string, progressCallback func(copied int64)) error {
	sFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dFile.Close()

	buf := make([]byte, 32*1024)
	var totalCopied int64

	for {
		n, rErr := sFile.Read(buf)
		if n > 0 {
			wN, wErr := dFile.Write(buf[:n])
			if wErr != nil {
				return wErr
			}
			totalCopied += int64(wN)
			if progressCallback != nil {
				progressCallback(totalCopied)
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return rErr
		}
	}
	return nil
}

func copyLocalDir(src, dst string, progressCallback func(copied, total int64)) error {
	var totalSize int64
	_ = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	var copiedSize int64
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		err = copyLocalFile(path, target, func(c int64) {
			if progressCallback != nil {
				progressCallback(copiedSize+c, totalSize)
			}
		})
		if err == nil {
			copiedSize += info.Size()
		}
		return err
	})
}

// GetProgress returns the current progress for a given job ID.
func (b *Bridge) GetProgress(jobID string) (Progress, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	prog, ok := b.jobs[jobID]
	if !ok {
		return Progress{
			JobID:  jobID,
			Status: "not_found",
		}, fmt.Errorf("job not found: %s", jobID)
	}

	return *prog, nil
}
