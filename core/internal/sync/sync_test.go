package sync_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/totalconnect/bridge"
	tcSync "github.com/totalconnect/core/internal/sync"
)

func TestCalculateDiff(t *testing.T) {
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	newer := now

	localFiles := []bridge.FileInfo{
		{Path: "local_only.txt", Size: 100, IsDir: false, ModTime: now},
		{Path: "in_sync.txt", Size: 200, IsDir: false, ModTime: now},
		{Path: "local_newer.txt", Size: 300, IsDir: false, ModTime: newer},
		{Path: "remote_newer.txt", Size: 400, IsDir: false, ModTime: older},
	}

	remoteFiles := []bridge.FileInfo{
		{Path: "remote_only.txt", Size: 500, IsDir: false, ModTime: now},
		{Path: "in_sync.txt", Size: 200, IsDir: false, ModTime: now},
		{Path: "local_newer.txt", Size: 300, IsDir: false, ModTime: older},
		{Path: "remote_newer.txt", Size: 400, IsDir: false, ModTime: newer},
	}

	diffs := tcSync.CalculateDiff(localFiles, remoteFiles, tcSync.StrategyNewestWins)

	diffMap := make(map[string]*tcSync.DiffItem)
	for _, d := range diffs {
		diffMap[d.Path] = d
	}

	if d, ok := diffMap["local_only.txt"]; !ok || d.Action != tcSync.ActionCreateRemote {
		t.Errorf("expected local_only.txt to have action create_remote, got %+v", d)
	}

	if d, ok := diffMap["remote_only.txt"]; !ok || d.Action != tcSync.ActionCreateLocal {
		t.Errorf("expected remote_only.txt to have action create_local, got %+v", d)
	}

	if d, ok := diffMap["in_sync.txt"]; !ok || d.Action != tcSync.ActionInSync {
		t.Errorf("expected in_sync.txt to have action in_sync, got %+v", d)
	}

	if d, ok := diffMap["local_newer.txt"]; !ok || d.Action != tcSync.ActionUpdateRemote {
		t.Errorf("expected local_newer.txt to have action update_remote, got %+v", d)
	}

	if d, ok := diffMap["remote_newer.txt"]; !ok || d.Action != tcSync.ActionUpdateLocal {
		t.Errorf("expected remote_newer.txt to have action update_local, got %+v", d)
	}
}

func TestConflictResolutionAskUser(t *testing.T) {
	now := time.Now()
	older := now.Add(-1 * time.Hour)

	localFiles := []bridge.FileInfo{
		{Path: "conflict_file.txt", Size: 100, IsDir: false, ModTime: now},
	}
	remoteFiles := []bridge.FileInfo{
		{Path: "conflict_file.txt", Size: 200, IsDir: false, ModTime: older},
	}

	diffs := tcSync.CalculateDiff(localFiles, remoteFiles, tcSync.StrategyAskUser)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(diffs))
	}

	if !diffs[0].Conflict || diffs[0].Action != tcSync.ActionConflict {
		t.Errorf("expected conflict action with strategy AskUser, got %+v", diffs[0])
	}
}

func TestBidirectionalSyncEngine(t *testing.T) {
	tempSrc := t.TempDir()
	tempDst := t.TempDir()

	// Create src file
	srcFile := filepath.Join(tempSrc, "file_src.txt")
	if err := os.WriteFile(srcFile, []byte("content from source"), 0644); err != nil {
		t.Fatalf("failed writing src file: %v", err)
	}

	// Create dst file
	dstFile := filepath.Join(tempDst, "file_dst.txt")
	if err := os.WriteFile(dstFile, []byte("content from destination"), 0644); err != nil {
		t.Fatalf("failed writing dst file: %v", err)
	}

	b := bridge.NewBridge()
	engine := tcSync.NewEngine(b)

	taskID, err := engine.Sync(context.Background(), tempSrc, tempDst, tcSync.PriorityNormal, tcSync.StrategyNewestWins, false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Wait for task completion
	var finalTask *tcSync.SyncTask
	for i := 0; i < 50; i++ {
		tProgress, pErr := engine.GetTaskProgress(taskID)
		if pErr != nil {
			t.Fatalf("GetTaskProgress failed: %v", pErr)
		}
		if tProgress.Status == "completed" || tProgress.Status == "failed" {
			finalTask = tProgress
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalTask == nil || finalTask.Status != "completed" {
		t.Fatalf("expected task status completed, got %+v", finalTask)
	}

	// Verify bidirectional sync: file_src.txt should be copied to tempDst, file_dst.txt should be copied to tempSrc
	if _, err := os.Stat(filepath.Join(tempDst, "file_src.txt")); err != nil {
		t.Errorf("file_src.txt missing from destination dir")
	}

	if _, err := os.Stat(filepath.Join(tempSrc, "file_dst.txt")); err != nil {
		t.Errorf("file_dst.txt missing from source dir")
	}
}

func TestConflictResolutionFlow(t *testing.T) {
	tempSrc := t.TempDir()
	tempDst := t.TempDir()

	file1Src := filepath.Join(tempSrc, "shared.txt")
	file1Dst := filepath.Join(tempDst, "shared.txt")

	_ = os.WriteFile(file1Src, []byte("source version"), 0644)
	_ = os.WriteFile(file1Dst, []byte("different destination version"), 0644)

	b := bridge.NewBridge()
	engine := tcSync.NewEngine(b)

	taskID, err := engine.Sync(context.Background(), tempSrc, tempDst, tcSync.PriorityNormal, tcSync.StrategyAskUser, false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	var conflictTask *tcSync.SyncTask
	for i := 0; i < 50; i++ {
		tProgress, _ := engine.GetTaskProgress(taskID)
		if tProgress != nil && tProgress.Status == "conflict" {
			conflictTask = tProgress
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if conflictTask == nil || conflictTask.Status != "conflict" {
		t.Fatalf("expected task status 'conflict', got %+v", conflictTask)
	}

	// Resolve conflict by choosing local
	err = engine.ResolveConflict(taskID, "shared.txt", "local")
	if err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	var finalTask *tcSync.SyncTask
	for i := 0; i < 50; i++ {
		tProgress, _ := engine.GetTaskProgress(taskID)
		if tProgress != nil && tProgress.Status == "completed" {
			finalTask = tProgress
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalTask == nil || finalTask.Status != "completed" {
		t.Fatalf("expected task status 'completed' after conflict resolution, got %+v", finalTask)
	}
}

func TestQueuePriority(t *testing.T) {
	b := bridge.NewBridge()
	engine := tcSync.NewEngine(b)

	taskIDLow, _ := engine.EnqueueTask(&tcSync.SyncTask{
		Source:      "srcLow",
		Destination: "dstLow",
		Priority:    tcSync.PriorityLow,
	})

	taskIDHigh, _ := engine.EnqueueTask(&tcSync.SyncTask{
		Source:      "srcHigh",
		Destination: "dstHigh",
		Priority:    tcSync.PriorityHigh,
	})

	pHigh, err1 := engine.GetTaskProgress(taskIDHigh)
	pLow, err2 := engine.GetTaskProgress(taskIDLow)

	if err1 != nil || err2 != nil {
		t.Fatalf("failed retrieving tasks: %v, %v", err1, err2)
	}

	if pHigh.Priority != tcSync.PriorityHigh || pLow.Priority != tcSync.PriorityLow {
		t.Errorf("priorities mismatched")
	}
}
