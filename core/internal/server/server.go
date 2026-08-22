package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	pb "github.com/totalconnect/api/v1"
	"github.com/totalconnect/bridge"
	tcsync "github.com/totalconnect/core/internal/sync"
	"github.com/totalconnect/core/internal/vault"
	"google.golang.org/grpc"
)

// DefaultAddr is the default listening address for the gRPC server.
const DefaultAddr = ":50051"

// VaultService implements the totalconnectv1.VaultServer gRPC service.
type VaultService struct {
	pb.UnimplementedVaultServer
	mu             sync.Mutex
	v              *vault.Vault
	lastPassphrase string
}

// NewVaultService returns a new VaultService backed by a Vault instance.
func NewVaultService(v *vault.Vault) *VaultService {
	if v == nil {
		v = vault.NewVault()
	}
	return &VaultService{v: v}
}

// Vault returns the underlying Vault.
func (s *VaultService) Vault() *vault.Vault {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.v == nil {
		s.v = vault.NewVault()
	}
	return s.v
}

func (s *VaultService) ensureUnlocked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.v == nil {
		s.v = vault.NewVault()
	}
	if !s.v.IsUnlocked() {
		pass := s.lastPassphrase
		if pass == "" {
			pass = "default"
		}
		_ = s.v.Unlock(pass)
	}
}

// Unlock unlocks the vault using the provided passphrase.
func (s *VaultService) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	if req.GetPassphrase() == "" {
		return &pb.UnlockResponse{
			Success: false,
			Message: "passphrase required",
		}, nil
	}
	s.mu.Lock()
	if s.v == nil {
		s.v = vault.NewVault()
	}
	err := s.v.Unlock(req.GetPassphrase())
	if err != nil {
		s.mu.Unlock()
		return &pb.UnlockResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	s.lastPassphrase = req.GetPassphrase()
	s.mu.Unlock()

	return &pb.UnlockResponse{
		Success: true,
		Message: "unlocked successfully",
	}, nil
}

// Lock locks the vault.
func (s *VaultService) Lock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	s.mu.Lock()
	if s.v != nil {
		s.v.Lock()
	}
	s.mu.Unlock()
	return &pb.LockResponse{
		Success: true,
	}, nil
}

// AddCredential adds a new credential to the vault and returns its ID.
func (s *VaultService) AddCredential(ctx context.Context, req *pb.AddCredentialRequest) (*pb.AddCredentialResponse, error) {
	s.ensureUnlocked()
	id, err := s.v.AddCredential(req.GetKey(), req.GetValue())
	if err != nil {
		return &pb.AddCredentialResponse{
			Success: false,
			Id:      "",
		}, nil
	}
	return &pb.AddCredentialResponse{
		Success: true,
		Id:      id,
	}, nil
}

// GetSSHKey retrieves an SSH key by name.
func (s *VaultService) GetSSHKey(ctx context.Context, req *pb.GetSSHKeyRequest) (*pb.GetSSHKeyResponse, error) {
	s.ensureUnlocked()
	keyName := req.GetKeyName()
	if keyName == "" {
		keyName = "default"
	}
	pubKey, privKey, err := s.v.GetSSHKey(keyName)
	if err != nil {
		return &pb.GetSSHKeyResponse{
			KeyName:    keyName,
			PublicKey:  "",
			PrivateKey: "",
		}, nil
	}
	return &pb.GetSSHKeyResponse{
		KeyName:    keyName,
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// FileService implements the totalconnectv1.FileServer gRPC service.
type FileService struct {
	pb.UnimplementedFileServer
	bridge     *bridge.Bridge
	syncEngine *tcsync.Engine
}

// NewFileService creates a new FileService instance.
func NewFileService(b *bridge.Bridge, engine *tcsync.Engine) *FileService {
	if b == nil {
		b = bridge.NewBridge()
	}
	if engine == nil {
		engine = tcsync.NewEngine(b)
	}
	return &FileService{
		bridge:     b,
		syncEngine: engine,
	}
}

// List lists files in the specified path.
func (s *FileService) List(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	path := req.GetPath()
	if path == "" {
		path = "."
	}

	files, err := s.bridge.ListFiles("local", path)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	pbFiles := make([]*pb.FileInfo, 0, len(files))
	for _, f := range files {
		pbFiles = append(pbFiles, &pb.FileInfo{
			Path:    f.Path,
			Size:    f.Size,
			IsDir:   f.IsDir,
			ModTime: f.ModTime.Unix(),
		})
	}

	return &pb.ListFilesResponse{
		Files: pbFiles,
	}, nil
}

// Transfer initiates a file transfer and returns a task ID.
func (s *FileService) Transfer(ctx context.Context, req *pb.TransferFileRequest) (*pb.TransferFileResponse, error) {
	jobID, err := s.bridge.Transfer(req.GetSource(), req.GetDestination(), "copy")
	if err != nil {
		return &pb.TransferFileResponse{
			Success: false,
			TaskId:  "",
		}, err
	}
	return &pb.TransferFileResponse{
		Success: true,
		TaskId:  jobID,
	}, nil
}

// Delete deletes a file or directory.
func (s *FileService) Delete(ctx context.Context, req *pb.DeleteFileRequest) (*pb.DeleteFileResponse, error) {
	path := req.GetPath()
	if path == "" {
		return &pb.DeleteFileResponse{Success: false}, fmt.Errorf("path required")
	}

	var err error
	if req.GetRecursive() {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return &pb.DeleteFileResponse{Success: false}, err
	}

	return &pb.DeleteFileResponse{
		Success: true,
	}, nil
}

// Sync syncs files between source and destination and returns a task ID.
func (s *FileService) Sync(ctx context.Context, req *pb.SyncFilesRequest) (*pb.SyncFilesResponse, error) {
	taskID, err := s.syncEngine.Sync(ctx, req.GetSource(), req.GetDestination(), tcsync.PriorityNormal, tcsync.StrategyNewestWins, req.GetDeleteExtraneous())
	if err != nil {
		return &pb.SyncFilesResponse{
			Success: false,
			TaskId:  "",
		}, err
	}
	return &pb.SyncFilesResponse{
		Success: true,
		TaskId:  taskID,
	}, nil
}

// StatusService implements the totalconnectv1.StatusServer gRPC service.
type StatusService struct {
	pb.UnimplementedStatusServer
	bridge     *bridge.Bridge
	syncEngine *tcsync.Engine
}

// NewStatusService creates a new StatusService instance.
func NewStatusService(b *bridge.Bridge, engine *tcsync.Engine) *StatusService {
	if b == nil {
		b = bridge.NewBridge()
	}
	if engine == nil {
		engine = tcsync.NewEngine(b)
	}
	return &StatusService{
		bridge:     b,
		syncEngine: engine,
	}
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
	taskID := req.GetTaskId()

	// Check Sync Engine first
	if task, err := s.syncEngine.GetTaskProgress(taskID); err == nil {
		return &pb.GetProgressResponse{
			TaskId:           task.TaskID,
			BytesTransferred: task.BytesTransferred,
			TotalBytes:       task.TotalBytes,
			Percentage:       task.Percentage,
			Status:           task.Status,
		}, nil
	}

	// Check Bridge
	if prog, err := s.bridge.GetProgress(taskID); err == nil {
		return &pb.GetProgressResponse{
			TaskId:           prog.JobID,
			BytesTransferred: prog.BytesTransferred,
			TotalBytes:       prog.TotalBytes,
			Percentage:       prog.Percentage,
			Status:           prog.Status,
		}, nil
	}

	return &pb.GetProgressResponse{
		TaskId:           taskID,
		BytesTransferred: 0,
		TotalBytes:       0,
		Percentage:       0.0,
		Status:           "not_found",
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
	bridge        *bridge.Bridge
	syncEngine    *tcsync.Engine
}

// NewServer creates a new Server instance with default or given address.
func NewServer(addr string) *Server {
	if addr == "" {
		addr = DefaultAddr
	}
	v := vault.NewVault()
	b := bridge.NewBridge()
	eng := tcsync.NewEngine(b)

	return &Server{
		addr:          addr,
		vaultService:  NewVaultService(v),
		fileService:   NewFileService(b, eng),
		statusService: NewStatusService(b, eng),
		bridge:        b,
		syncEngine:    eng,
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

// Bridge returns the attached Bridge.
func (s *Server) Bridge() *bridge.Bridge {
	return s.bridge
}

// SyncEngine returns the attached SyncEngine.
func (s *Server) SyncEngine() *tcsync.Engine {
	return s.syncEngine
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
