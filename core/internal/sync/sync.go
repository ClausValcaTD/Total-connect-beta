package sync

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/totalconnect/bridge"
)

type DiffAction string

const (
	ActionCreateLocal  DiffAction = "create_local"
	ActionCreateRemote DiffAction = "create_remote"
	ActionUpdateLocal  DiffAction = "update_local"
	ActionUpdateRemote DiffAction = "update_remote"
	ActionConflict     DiffAction = "conflict"
	ActionInSync       DiffAction = "in_sync"
	ActionDeleteLocal  DiffAction = "delete_local"
	ActionDeleteRemote DiffAction = "delete_remote"
)

type ConflictStrategy string

const (
	StrategyNewestWins ConflictStrategy = "newest_wins"
	StrategyAskUser    ConflictStrategy = "ask_user"
)

type Priority int

const (
	PriorityHigh   Priority = 1
	PriorityNormal Priority = 2
	PriorityLow    Priority = 3
)

// DiffItem represents the difference analysis for a single file or directory.
type DiffItem struct {
	Path       string           `json:"path"`
	LocalFile  *bridge.FileInfo `json:"local_file,omitempty"`
	RemoteFile *bridge.FileInfo `json:"remote_file,omitempty"`
	Action     DiffAction       `json:"action"`
	Conflict   bool             `json:"conflict"`
	Resolution string           `json:"resolution,omitempty"` // "local", "remote"
}

// SyncTask represents a single sync operation.
type SyncTask struct {
	TaskID           string           `json:"task_id"`
	Source           string           `json:"source"`
	Destination      string           `json:"destination"`
	SourceRemote     string           `json:"source_remote"`
	DestRemote       string           `json:"dest_remote"`
	Priority         Priority         `json:"priority"`
	Strategy         ConflictStrategy `json:"strategy"`
	DeleteExtraneous bool             `json:"delete_extraneous"`
	Status           string           `json:"status"` // "queued", "in_progress", "completed", "failed", "conflict"
	TotalFiles       int64            `json:"total_files"`
	ProcessedFiles   int64            `json:"processed_files"`
	TotalBytes       int64            `json:"total_bytes"`
	BytesTransferred int64            `json:"bytes_transferred"`
	Percentage       float64          `json:"percentage"`
	Diffs            []*DiffItem      `json:"diffs"`
	Error            string           `json:"error,omitempty"`
}

// Engine manages bidirectional sync operations, diff calculation, queue priorities, and conflict resolution.
type Engine struct {
	mu          sync.RWMutex
	bridge      *bridge.Bridge
	queue       []*SyncTask
	tasks       map[string]*SyncTask
	taskCounter int64
}

// NewEngine returns a new instance of Sync Engine.
func NewEngine(b *bridge.Bridge) *Engine {
	if b == nil {
		b = bridge.NewBridge()
	}
	return &Engine{
		bridge: b,
		tasks:  make(map[string]*SyncTask),
	}
}

// CalculateDiff performs smart diff algorithm between local and remote file lists.
func CalculateDiff(localFiles, remoteFiles []bridge.FileInfo, strategy ConflictStrategy) []*DiffItem {
	localMap := make(map[string]bridge.FileInfo)
	for _, f := range localFiles {
		clean := filepath.Clean(f.Path)
		localMap[clean] = f
	}

	remoteMap := make(map[string]bridge.FileInfo)
	for _, f := range remoteFiles {
		clean := filepath.Clean(f.Path)
		remoteMap[clean] = f
	}

	allPathsMap := make(map[string]bool)
	for p := range localMap {
		allPathsMap[p] = true
	}
	for p := range remoteMap {
		allPathsMap[p] = true
	}

	var allPaths []string
	for p := range allPathsMap {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	var diffs []*DiffItem
	for _, path := range allPaths {
		loc, hasLoc := localMap[path]
		rem, hasRem := remoteMap[path]

		item := &DiffItem{Path: path}

		if hasLoc && !hasRem {
			lCopy := loc
			item.LocalFile = &lCopy
			item.Action = ActionCreateRemote
		} else if !hasLoc && hasRem {
			rCopy := rem
			item.RemoteFile = &rCopy
			item.Action = ActionCreateLocal
		} else {
			lCopy := loc
			rCopy := rem
			item.LocalFile = &lCopy
			item.RemoteFile = &rCopy

			if loc.IsDir && rem.IsDir {
				item.Action = ActionInSync
			} else if loc.IsDir != rem.IsDir {
				item.Conflict = true
				item.Action = ActionConflict
			} else {
				// File comparison: size and modtime
				modDiff := math.Abs(loc.ModTime.Sub(rem.ModTime).Seconds())
				if loc.Size == rem.Size && modDiff < 2.0 {
					item.Action = ActionInSync
				} else {
					if strategy == StrategyAskUser {
						item.Conflict = true
						item.Action = ActionConflict
					} else { // StrategyNewestWins
						if loc.ModTime.After(rem.ModTime) {
							item.Action = ActionUpdateRemote
						} else if rem.ModTime.After(loc.ModTime) {
							item.Action = ActionUpdateLocal
						} else {
							item.Action = ActionInSync
						}
					}
				}
			}
		}

		diffs = append(diffs, item)
	}

	return diffs
}

// EnqueueTask adds a new sync task to the priority queue.
func (e *Engine) EnqueueTask(task *SyncTask) (string, error) {
	e.mu.Lock()
	if task.TaskID == "" {
		id := atomic.AddInt64(&e.taskCounter, 1)
		task.TaskID = fmt.Sprintf("task-sync-%d", id)
	}
	if task.Priority == 0 {
		task.Priority = PriorityNormal
	}
	if task.Strategy == "" {
		task.Strategy = StrategyNewestWins
	}
	task.Status = "queued"

	e.tasks[task.TaskID] = task
	e.queue = append(e.queue, task)

	// Sort queue by priority (1 = High < 2 = Normal < 3 = Low)
	sort.SliceStable(e.queue, func(i, j int) bool {
		return e.queue[i].Priority < e.queue[j].Priority
	})
	e.mu.Unlock()

	go e.processNextInQueue()

	return task.TaskID, nil
}

// Sync is a convenience method to create, queue, and process a sync operation.
func (e *Engine) Sync(ctx context.Context, source, destination string, priority Priority, strategy ConflictStrategy, deleteExtraneous bool) (string, error) {
	task := &SyncTask{
		Source:           source,
		Destination:      destination,
		Priority:         priority,
		Strategy:         strategy,
		DeleteExtraneous: deleteExtraneous,
	}
	return e.EnqueueTask(task)
}

// GetTaskProgress returns current status and progress of a sync task.
func (e *Engine) GetTaskProgress(taskID string) (*SyncTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// Return shallow copy for safety
	cp := *task
	return &cp, nil
}

// ResolveConflict resolves a manual conflict for a specific path in a task.
func (e *Engine) ResolveConflict(taskID, path string, choice string) error {
	e.mu.Lock()
	task, ok := e.tasks[taskID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	cleanPath := filepath.Clean(path)
	found := false
	hasRemainingConflicts := false

	for _, d := range task.Diffs {
		if filepath.Clean(d.Path) == cleanPath && d.Conflict {
			found = true
			d.Conflict = false
			d.Resolution = choice

			if choice == "local" {
				if d.RemoteFile == nil {
					d.Action = ActionCreateRemote
				} else {
					d.Action = ActionUpdateRemote
				}
			} else if choice == "remote" {
				if d.LocalFile == nil {
					d.Action = ActionCreateLocal
				} else {
					d.Action = ActionUpdateLocal
				}
			} else {
				d.Action = ActionInSync
			}
		}
		if d.Conflict {
			hasRemainingConflicts = true
		}
	}

	if !found {
		e.mu.Unlock()
		return fmt.Errorf("conflict path '%s' not found or already resolved", path)
	}

	if !hasRemainingConflicts && task.Status == "conflict" {
		task.Status = "in_progress"
		e.mu.Unlock()
		go e.executeTaskActions(task)
		return nil
	}

	e.mu.Unlock()
	return nil
}

func (e *Engine) processNextInQueue() {
	e.mu.Lock()
	var taskToRun *SyncTask
	for i, task := range e.queue {
		if task.Status == "queued" {
			taskToRun = task
			// Remove from queue
			e.queue = append(e.queue[:i], e.queue[i+1:]...)
			break
		}
	}
	if taskToRun == nil {
		e.mu.Unlock()
		return
	}
	taskToRun.Status = "in_progress"
	e.mu.Unlock()

	e.runSyncTask(taskToRun)
}

func (e *Engine) runSyncTask(task *SyncTask) {
	localFiles, locErr := e.bridge.ListFiles(task.SourceRemote, task.Source)
	if locErr != nil {
		locEntries, _ := os.ReadDir(task.Source)
		for _, entry := range locEntries {
			info, _ := entry.Info()
			var sz int64
			var mt time.Time
			if info != nil {
				sz = info.Size()
				mt = info.ModTime()
			}
			localFiles = append(localFiles, bridge.FileInfo{
				Path:    entry.Name(),
				Size:    sz,
				IsDir:   entry.IsDir(),
				ModTime: mt,
			})
		}
	}

	remoteFiles, remErr := e.bridge.ListFiles(task.DestRemote, task.Destination)
	if remErr != nil {
		remEntries, _ := os.ReadDir(task.Destination)
		for _, entry := range remEntries {
			info, _ := entry.Info()
			var sz int64
			var mt time.Time
			if info != nil {
				sz = info.Size()
				mt = info.ModTime()
			}
			remoteFiles = append(remoteFiles, bridge.FileInfo{
				Path:    entry.Name(),
				Size:    sz,
				IsDir:   entry.IsDir(),
				ModTime: mt,
			})
		}
	}

	diffs := CalculateDiff(localFiles, remoteFiles, task.Strategy)

	e.mu.Lock()
	task.Diffs = diffs
	task.TotalFiles = int64(len(diffs))
	for _, d := range diffs {
		if d.LocalFile != nil {
			task.TotalBytes += d.LocalFile.Size
		} else if d.RemoteFile != nil {
			task.TotalBytes += d.RemoteFile.Size
		}
	}

	// Check if conflicts exist
	hasConflict := false
	for _, d := range diffs {
		if d.Conflict {
			hasConflict = true
			break
		}
	}

	if hasConflict {
		task.Status = "conflict"
		e.mu.Unlock()
		return
	}
	e.mu.Unlock()

	e.executeTaskActions(task)
}

func (e *Engine) executeTaskActions(task *SyncTask) {
	for _, d := range task.Diffs {
		if d.Action == ActionInSync {
			e.mu.Lock()
			task.ProcessedFiles++
			if task.TotalFiles > 0 {
				task.Percentage = (float64(task.ProcessedFiles) / float64(task.TotalFiles)) * 100.0
			}
			e.mu.Unlock()
			continue
		}

		srcPath := filepath.Join(task.Source, d.Path)
		dstPath := filepath.Join(task.Destination, d.Path)

		switch d.Action {
		case ActionCreateRemote, ActionUpdateRemote:
			jobID, err := e.bridge.Transfer(srcPath, dstPath, "upload")
			if err == nil {
				e.waitBridgeJob(task, jobID, d)
			}
		case ActionCreateLocal, ActionUpdateLocal:
			jobID, err := e.bridge.Transfer(dstPath, srcPath, "download")
			if err == nil {
				e.waitBridgeJob(task, jobID, d)
			}
		}

		e.mu.Lock()
		task.ProcessedFiles++
		if task.TotalFiles > 0 {
			task.Percentage = (float64(task.ProcessedFiles) / float64(task.TotalFiles)) * 100.0
		}
		e.mu.Unlock()
	}

	e.mu.Lock()
	task.Status = "completed"
	task.Percentage = 100.0
	e.mu.Unlock()

	go e.processNextInQueue()
}

func (e *Engine) waitBridgeJob(task *SyncTask, jobID string, item *DiffItem) {
	for i := 0; i < 50; i++ {
		prog, err := e.bridge.GetProgress(jobID)
		if err == nil && (prog.Status == "completed" || prog.Status == "failed") {
			e.mu.Lock()
			task.BytesTransferred += prog.BytesTransferred
			e.mu.Unlock()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
}
