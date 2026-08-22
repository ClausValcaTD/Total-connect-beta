package tui

import (
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
}

// InitialModel constructs and initializes a default Model.
func InitialModel() Model {
	styles := DefaultStyles()

	p := progress.New(progress.WithDefaultGradient())
	p.Width = 30

	ti := textinput.New()
	ti.Placeholder = "Enter master passphrase"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 64

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	m := Model{
		ActivePane:      PaneLocal,
		Styles:          styles,
		ProgressBar:     p,
		PassphraseInput: ti,
		GrpcAddr:        ":50051",
		LocalPane: Pane{
			Type:        PaneLocal,
			CurrentPath: cwd,
			Cursor:      0,
		},
		RemotePane: Pane{
			Type:        PaneRemote,
			CurrentPath: "/",
			Cursor:      0,
		},
		StatusMsg: "Press F2 to Unlock Vault | F5 Copy | Tab Switch Pane | F10 Quit",
	}

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
