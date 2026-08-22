package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	pb "github.com/totalconnect/api/v1"
	"google.golang.org/grpc"
)

// PaneType represents whether a pane is Local or Remote.
type PaneType int

const (
	PaneLocal PaneType = iota
	PaneRemote
)

// FileItem represents a file or directory in a pane.
type FileItem struct {
	Name    string
	Path    string
	Size    int64
	IsDir   bool
	ModTime time.Time
}

// Pane represents one side of the dual-pane interface.
type Pane struct {
	Type        PaneType
	CurrentPath string
	Files       []FileItem
	Cursor      int
	Selected    map[string]bool
}

// Model represents the main Bubble Tea model for Total Commander TUI.
type Model struct {
	Width             int
	Height            int
	ActivePane        PaneType
	LocalPane         Pane
	RemotePane        Pane
	Styles            Styles
	ProgressBar       progress.Model
	PassphraseInput   textinput.Model
	ShowingVaultModal bool
	ShowingCopyDialog  bool // FIX #7: confirmation dialog for F5
	CopyDialogSrc      string
	CopyDialogDst      string
	ShowingMkDirModal  bool
	MkDirInput         textinput.Model
	GrpcAddr          string
	GrpcConnected     bool
	GrpcConn          *grpc.ClientConn
	VaultClient       pb.VaultClient
	FileClient        pb.FileClient
	StatusClient      pb.StatusClient
	VaultUnlocked     bool
	StatusMsg         string
	TransferActive    bool
	TransferProgress  float64
	TaskId            string

	Profiles             []Profile
	ShowingProfilesModal bool
	ProfileCursor        int
}

// NewClient connects to the gRPC server and returns a ready Model.
// FIX #1 + #2: replaces the missing NewClient / NewModel pair.
func NewClient(addr string) (*Model, error) {
	m := initialModel(addr)
	return &m, nil
}

// NewModel wraps an existing *Model pointer for tea.NewProgram.
// main.go calls tui.NewModel(client) where client is *Model.
func NewModel(m *Model) Model {
	return *m
}

// InitialModel builds the zero-state Model for default address or custom address.
func InitialModel(addr ...string) Model {
	a := ":50051"
	if len(addr) > 0 && addr[0] != "" {
		a = addr[0]
	}
	return initialModel(a)
}

// initialModel builds the zero-state Model before any gRPC connection.
func initialModel(addr string) Model {
	styles := DefaultStyles()

	p := progress.New(progress.WithDefaultGradient())
	p.Width = 30

	ti := textinput.New()
	ti.Placeholder = "Enter master passphrase"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 64

	mkdir := textinput.New()
	mkdir.Placeholder = "New directory name"
	mkdir.CharLimit = 128

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	m := Model{
		ActivePane:      PaneLocal,
		Styles:          styles,
		ProgressBar:     p,
		PassphraseInput: ti,
		MkDirInput:      mkdir,
		GrpcAddr:        addr,
		LocalPane: Pane{
			Type:        PaneLocal,
			CurrentPath: cwd,
			Cursor:      0,
			Selected:    make(map[string]bool),
		},
		RemotePane: Pane{
			Type:        PaneRemote,
			CurrentPath: "/",
			Cursor:      0,
			Selected:    make(map[string]bool),
		},
		StatusMsg: "Press F2 to Unlock Vault | F5 Copy | Tab Switch Pane | F10 Quit",
	}

	profiles, _ := LoadProfiles()
	m.Profiles = profiles

	m.LoadLocalFiles()
	return m
}

// LoadLocalFiles populates the LocalPane file list from disk.
func (m *Model) LoadLocalFiles() {
	entries, err := os.ReadDir(m.LocalPane.CurrentPath)
	if err != nil {
		m.LocalPane.Files = []FileItem{
			{Name: "..", Path: filepath.Dir(m.LocalPane.CurrentPath), IsDir: true},
		}
		return
	}

	files := []FileItem{}
	if m.LocalPane.CurrentPath != "/" && m.LocalPane.CurrentPath != "." {
		files = append(files, FileItem{
			Name:  "..",
			Path:  filepath.Dir(m.LocalPane.CurrentPath),
			IsDir: true,
		})
	}

	for _, entry := range entries {
		info, err := entry.Info()
		size := int64(0)
		modTime := time.Now()
		if err == nil {
			size = info.Size()
			modTime = info.ModTime()
		}

		files = append(files, FileItem{
			Name:    entry.Name(),
			Path:    filepath.Join(m.LocalPane.CurrentPath, entry.Name()),
			Size:    size,
			IsDir:   entry.IsDir(),
			ModTime: modTime,
		})
	}

	m.LocalPane.Files = files
	if m.LocalPane.Cursor >= len(files) {
		m.LocalPane.Cursor = 0
	}
}

// GetActivePane returns pointer to currently focused Pane.
func (m *Model) GetActivePane() *Pane {
	if m.ActivePane == PaneLocal {
		return &m.LocalPane
	}
	return &m.RemotePane
}

// GetInactivePane returns pointer to the unfocused Pane.
func (m *Model) GetInactivePane() *Pane {
	if m.ActivePane == PaneLocal {
		return &m.RemotePane
	}
	return &m.LocalPane
}

// FormatSize converts bytes to a human-readable string.
// FIX #6: replaces raw "%8d B" formatting.
func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%6d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%6.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
