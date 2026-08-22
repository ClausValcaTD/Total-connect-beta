package server_test

import (
	"context"
	"net"
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

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
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

func TestFileService(t *testing.T) {
	_, _, fileClient, _, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// List
	listResp, err := fileClient.List(ctx, &pb.ListFilesRequest{Path: "/test/dir"})
	if err != nil {
		t.Fatalf("List RPC failed: %v", err)
	}
	if len(listResp.GetFiles()) == 0 {
		t.Errorf("expected non-empty file list response")
	}

	// Transfer
	transferResp, err := fileClient.Transfer(ctx, &pb.TransferFileRequest{
		Source:      "/local/path/file.txt",
		Destination: "/remote/path/file.txt",
	})
	if err != nil {
		t.Fatalf("Transfer RPC failed: %v", err)
	}
	if !transferResp.GetSuccess() || transferResp.GetTaskId() == "" {
		t.Errorf("expected transfer success with job_id, got success=%v, task_id=%s", transferResp.GetSuccess(), transferResp.GetTaskId())
	}

	// Delete
	deleteResp, err := fileClient.Delete(ctx, &pb.DeleteFileRequest{Path: "/test/dir/file.txt"})
	if err != nil {
		t.Fatalf("Delete RPC failed: %v", err)
	}
	if !deleteResp.GetSuccess() {
		t.Errorf("expected delete success, got false")
	}

	// Sync
	syncResp, err := fileClient.Sync(ctx, &pb.SyncFilesRequest{
		Source:      "/local/dir",
		Destination: "/remote/dir",
	})
	if err != nil {
		t.Fatalf("Sync RPC failed: %v", err)
	}
	if !syncResp.GetSuccess() || syncResp.GetTaskId() == "" {
		t.Errorf("expected sync success with job_id, got success=%v, task_id=%s", syncResp.GetSuccess(), syncResp.GetTaskId())
	}
}

func TestStatusService(t *testing.T) {
	_, _, _, statusClient, cleanup := startTestServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Health
	healthResp, err := statusClient.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health RPC failed: %v", err)
	}
	if healthResp.GetStatus() != "ok" {
		t.Errorf("expected status 'ok', got '%s'", healthResp.GetStatus())
	}

	// GetProgress
	progressResp, err := statusClient.GetProgress(ctx, &pb.GetProgressRequest{TaskId: "job-123"})
	if err != nil {
		t.Fatalf("GetProgress RPC failed: %v", err)
	}
	if progressResp.GetTaskId() != "job-123" || progressResp.GetStatus() == "" {
		t.Errorf("unexpected progress response: %+v", progressResp)
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
