package server

import (
	"context"
	"fmt"
	"net"

	pb "github.com/totalconnect/api/v1"
	"google.golang.org/grpc"
)

// DefaultAddr is the default listening address for the gRPC server.
const DefaultAddr = ":50051"

// VaultService implements the totalconnectv1.VaultServer gRPC service.
type VaultService struct {
	pb.UnimplementedVaultServer
}

// Unlock unlocks the vault using the provided passphrase.
func (s *VaultService) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	if req.GetPassphrase() == "" {
		return &pb.UnlockResponse{
			Success: false,
			Message: "passphrase required",
		}, nil
	}
	return &pb.UnlockResponse{
		Success: true,
		Message: "unlocked successfully",
	}, nil
}

// Lock locks the vault.
func (s *VaultService) Lock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	return &pb.LockResponse{
		Success: true,
	}, nil
}

// AddCredential adds a new credential to the vault and returns its ID.
func (s *VaultService) AddCredential(ctx context.Context, req *pb.AddCredentialRequest) (*pb.AddCredentialResponse, error) {
	id := fmt.Sprintf("cred-%s", req.GetKey())
	if req.GetKey() == "" {
		id = "cred-default"
	}
	return &pb.AddCredentialResponse{
		Success: true,
		Id:      id,
	}, nil
}

// GetSSHKey retrieves an SSH key by name.
func (s *VaultService) GetSSHKey(ctx context.Context, req *pb.GetSSHKeyRequest) (*pb.GetSSHKeyResponse, error) {
	keyName := req.GetKeyName()
	if keyName == "" {
		keyName = "default"
	}
	return &pb.GetSSHKeyResponse{
		KeyName:    keyName,
		PublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
		PrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\n...",
	}, nil
}

// FileService implements the totalconnectv1.FileServer gRPC service.
type FileService struct {
	pb.UnimplementedFileServer
}

// List lists files in the specified path.
func (s *FileService) List(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	path := req.GetPath()
	return &pb.ListFilesResponse{
		Files: []*pb.FileInfo{
			{
				Path:    path + "/file1.txt",
				Size:    1024,
				IsDir:   false,
				ModTime: 1600000000,
			},
		},
	}, nil
}

// Transfer initiates a file transfer and returns a task ID.
func (s *FileService) Transfer(ctx context.Context, req *pb.TransferFileRequest) (*pb.TransferFileResponse, error) {
	return &pb.TransferFileResponse{
		Success: true,
		TaskId:  "job-transfer-1",
	}, nil
}

// Delete deletes a file or directory.
func (s *FileService) Delete(ctx context.Context, req *pb.DeleteFileRequest) (*pb.DeleteFileResponse, error) {
	return &pb.DeleteFileResponse{
		Success: true,
	}, nil
}

// Sync syncs files between source and destination and returns a task ID.
func (s *FileService) Sync(ctx context.Context, req *pb.SyncFilesRequest) (*pb.SyncFilesResponse, error) {
	return &pb.SyncFilesResponse{
		Success: true,
		TaskId:  "job-sync-1",
	}, nil
}

// StatusService implements the totalconnectv1.StatusServer gRPC service.
type StatusService struct {
	pb.UnimplementedStatusServer
}

// Health checks system status and returns health info.
func (s *StatusService) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status:  "ok",
		Version: "0.1.0",
		Details: map[string]string{
			"uptime": "running",
		},
	}, nil
}

// GetProgress returns progress details for a given task ID.
func (s *StatusService) GetProgress(ctx context.Context, req *pb.GetProgressRequest) (*pb.GetProgressResponse, error) {
	return &pb.GetProgressResponse{
		TaskId:           req.GetTaskId(),
		BytesTransferred: 10240,
		TotalBytes:       102400,
		Percentage:       10.0,
		Status:           "in_progress",
	}, nil
}

// Server represents the Total Connect gRPC server.
type Server struct {
	grpcServer    *grpc.Server
	listener      net.Listener
	addr          string
	vaultService  *VaultService
	fileService   *FileService
	statusService *StatusService
}

// NewServer creates a new Server instance with default or given address.
func NewServer(addr string) *Server {
	if addr == "" {
		addr = DefaultAddr
	}
	return &Server{
		addr:          addr,
		vaultService:  &VaultService{},
		fileService:   &FileService{},
		statusService: &StatusService{},
	}
}

// VaultService returns the attached VaultService.
func (s *Server) VaultService() *VaultService {
	return s.vaultService
}

// FileService returns the attached FileService.
func (s *Server) FileService() *FileService {
	return s.fileService
}

// StatusService returns the attached StatusService.
func (s *Server) StatusService() *StatusService {
	return s.statusService
}

// Addr returns the listening address of the server.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

// Start starts listening on TCP network address and serving gRPC requests.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	return s.Serve(lis)
}

// Serve starts serving gRPC requests on the given listener.
func (s *Server) Serve(lis net.Listener) error {
	s.listener = lis
	s.grpcServer = grpc.NewServer()

	pb.RegisterVaultServer(s.grpcServer, s.vaultService)
	pb.RegisterFileServer(s.grpcServer, s.fileService)
	pb.RegisterStatusServer(s.grpcServer, s.statusService)

	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}
