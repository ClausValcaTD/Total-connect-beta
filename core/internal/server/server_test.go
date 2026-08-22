package server_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/totalconnect/api/v1"
	"github.com/totalconnect/core/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func startTestServer(t *testing.T) (*server.Server, pb.VaultClient, pb.FileClient, pb.StatusClient, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}

	srv := server.NewServer(lis.Addr().String())

	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		srv.Stop()
		t.Fatalf("failed to connect to server: %v", err)
	}

	vaultClient := pb.NewVaultClient(conn)
	fileClient := pb.NewFileClient(conn)
	statusClient := pb.NewStatusClient(conn)

	cleanup := func() {
		conn.Close()
		srv.Stop()
	}

	return srv, vaultClient, fileClient, statusClient, cleanup
}

func TestDefaultServerAddress(t *testing.T) {
	srv := server.NewServer("")
	if srv.Addr() != server.DefaultAddr {
		t.Errorf("expected default addr %s, got %s", server.DefaultAddr, srv.Addr())
	}
}

func TestVaultService(t *testing.T) {
	_, vaultClient, _, _, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Unlock with passphrase
	unlockResp, err := vaultClient.Unlock(ctx, &pb.UnlockRequest{Passphrase: "secret-password"})
	if err != nil {
		t.Fatalf("Unlock RPC failed: %v", err)
	}
	if !unlockResp.GetSuccess() {
		t.Errorf("expected unlock success, got false")
	}

	// Unlock with empty passphrase
	unlockRespFail, err := vaultClient.Unlock(ctx, &pb.UnlockRequest{Passphrase: ""})
	if err != nil {
		t.Fatalf("Unlock RPC failed: %v", err)
	}
	if unlockRespFail.GetSuccess() {
		t.Errorf("expected unlock failure for empty passphrase, got true")
	}

	// Lock
	lockResp, err := vaultClient.Lock(ctx, &pb.LockRequest{})
	if err != nil {
		t.Fatalf("Lock RPC failed: %v", err)
	}
	if !lockResp.GetSuccess() {
		t.Errorf("expected lock success, got false")
	}

	// AddCredential
	addResp, err := vaultClient.AddCredential(ctx, &pb.AddCredentialRequest{
		Key:   "db_password",
		Value: "supersecret",
	})
	if err != nil {
		t.Fatalf("AddCredential RPC failed: %v", err)
	}
	if !addResp.GetSuccess() || addResp.GetId() == "" {
		t.Errorf("expected credential addition success with non-empty id, got success=%v, id=%s", addResp.GetSuccess(), addResp.GetId())
	}

	// GetSSHKey
	keyResp, err := vaultClient.GetSSHKey(ctx, &pb.GetSSHKeyRequest{KeyName: "id_rsa"})
	if err != nil {
		t.Fatalf("GetSSHKey RPC failed: %v", err)
	}
	if keyResp.GetKeyName() != "id_rsa" || keyResp.GetPublicKey() == "" {
		t.Errorf("unexpected ssh key response: %+v", keyResp)
	}
}

func TestFileServiceAndStatusServiceWithBridgeAndSync(t *testing.T) {
	_, _, fileClient, statusClient, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	file1Path := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(file1Path, []byte("hello bridge sync"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. List files via bridge
	listResp, err := fileClient.List(ctx, &pb.ListFilesRequest{Path: tempDir})
	if err != nil {
		t.Fatalf("List RPC failed: %v", err)
	}
	if len(listResp.GetFiles()) == 0 {
		t.Errorf("expected non-empty file list response")
	}

	// 2. Transfer file via bridge
	tempDstDir := t.TempDir()
	dstFile := filepath.Join(tempDstDir, "sample.txt")
	transferResp, err := fileClient.Transfer(ctx, &pb.TransferFileRequest{
		Source:      file1Path,
		Destination: dstFile,
	})
	if err != nil {
		t.Fatalf("Transfer RPC failed: %v", err)
	}
	if !transferResp.GetSuccess() || transferResp.GetTaskId() == "" {
		t.Errorf("expected transfer success with job_id, got success=%v, task_id=%s", transferResp.GetSuccess(), transferResp.GetTaskId())
	}

	// Wait and check transfer progress via StatusService
	var transferProg *pb.GetProgressResponse
	for i := 0; i < 50; i++ {
		progResp, pErr := statusClient.GetProgress(ctx, &pb.GetProgressRequest{TaskId: transferResp.GetTaskId()})
		if pErr == nil && (progResp.GetStatus() == "completed" || progResp.GetStatus() == "failed") {
			transferProg = progResp
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if transferProg == nil || transferProg.GetStatus() != "completed" {
		t.Errorf("expected transfer status 'completed', got %+v", transferProg)
	}

	// 3. Sync local directory via Sync Engine
	syncSrcDir := t.TempDir()
	syncDstDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(syncSrcDir, "sync1.txt"), []byte("sync payload"), 0644)

	syncResp, err := fileClient.Sync(ctx, &pb.SyncFilesRequest{
		Source:      syncSrcDir,
		Destination: syncDstDir,
	})
	if err != nil {
		t.Fatalf("Sync RPC failed: %v", err)
	}
	if !syncResp.GetSuccess() || syncResp.GetTaskId() == "" {
		t.Errorf("expected sync success with task_id, got success=%v, task_id=%s", syncResp.GetSuccess(), syncResp.GetTaskId())
	}

	// Wait and check sync progress via StatusService
	var syncProg *pb.GetProgressResponse
	for i := 0; i < 50; i++ {
		progResp, pErr := statusClient.GetProgress(ctx, &pb.GetProgressRequest{TaskId: syncResp.GetTaskId()})
		if pErr == nil && (progResp.GetStatus() == "completed" || progResp.GetStatus() == "failed") {
			syncProg = progResp
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if syncProg == nil || syncProg.GetStatus() != "completed" {
		t.Errorf("expected sync status 'completed', got %+v", syncProg)
	}

	// Verify file was synced
	if _, err := os.Stat(filepath.Join(syncDstDir, "sync1.txt")); err != nil {
		t.Errorf("synced file missing in destination directory")
	}

	// 4. Delete file
	deleteFile := filepath.Join(tempDir, "to_delete.txt")
	_ = os.WriteFile(deleteFile, []byte("delete me"), 0644)
	deleteResp, err := fileClient.Delete(ctx, &pb.DeleteFileRequest{Path: deleteFile})
	if err != nil {
		t.Fatalf("Delete RPC failed: %v", err)
	}
	if !deleteResp.GetSuccess() {
		t.Errorf("expected delete success, got false")
	}
}

func TestStatusServiceHealth(t *testing.T) {
	_, _, _, statusClient, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthResp, err := statusClient.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health RPC failed: %v", err)
	}
	if healthResp.GetStatus() != "ok" {
		t.Errorf("expected status 'ok', got '%s'", healthResp.GetStatus())
	}
}

func TestServerPort50051Binding(t *testing.T) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		t.Skipf("skipping :50051 test if port is in use: %v", err)
	}

	srv := server.NewServer(":50051")
	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	conn, err := grpc.NewClient(":50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to :50051: %v", err)
	}
	defer conn.Close()

	client := pb.NewStatusClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := client.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health call on :50051 failed: %v", err)
	}
	if res.GetStatus() != "ok" {
		t.Errorf("expected ok status, got %s", res.GetStatus())
	}
}
