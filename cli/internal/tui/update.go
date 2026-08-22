package tui

import (
	"context"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	pb "github.com/totalconnect/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Message types for Bubble Tea event loop
type grpcConnectedMsg struct {
	conn         *grpc.ClientConn
	vaultClient  pb.VaultClient
	fileClient   pb.FileClient
	statusClient pb.StatusClient
	err          error
}

type remoteFilesMsg struct {
	files []FileItem
	err   error
}

type vaultUnlockedMsg struct {
	success bool
	msg     string
}

type vaultLockedMsg struct {
	success bool
}

type transferStartedMsg struct {
	taskId  string
	success bool
	msg     string
}

type progressMsg struct {
	percentage float64
	status     string
	done       bool
}

type tickMsg time.Time

// Init initializes background commands like gRPC connection attempt.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.connectGrpcCmd(),
		textinput.Blink,
	)
}

func (m Model) connectGrpcCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, m.GrpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err != nil {
			return grpcConnectedMsg{err: err}
		}

		return grpcConnectedMsg{
			conn:         conn,
			vaultClient:  pb.NewVaultClient(conn),
			fileClient:   pb.NewFileClient(conn),
			statusClient: pb.NewStatusClient(conn),
		}
	}
}

func (m Model) listRemoteFilesCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if m.FileClient == nil {
			return remoteFilesMsg{err: grpc.ErrServerStopped}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		res, err := m.FileClient.List(ctx, &pb.ListFilesRequest{Path: path})
		if err != nil {
			return remoteFilesMsg{err: err}
		}

		items := []FileItem{}
		if path != "/" && path != "" {
			items = append(items, FileItem{
				Name:  "..",
				Path:  filepath.Dir(path),
				IsDir: true,
			})
		}

		for _, f := range res.GetFiles() {
			items = append(items, FileItem{
				Name:    filepath.Base(f.GetPath()),
				Path:    f.GetPath(),
				Size:    f.GetSize(),
				IsDir:   f.GetIsDir(),
				ModTime: time.Unix(f.GetModTime(), 0),
			})
		}

		return remoteFilesMsg{files: items}
	}
}

func (m Model) unlockVaultCmd(passphrase string) tea.Cmd {
	return func() tea.Msg {
		if m.VaultClient == nil {
			return vaultUnlockedMsg{success: false, msg: "gRPC server not connected"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		res, err := m.VaultClient.Unlock(ctx, &pb.UnlockRequest{Passphrase: passphrase})
		if err != nil {
			return vaultUnlockedMsg{success: false, msg: err.Error()}
		}

		return vaultUnlockedMsg{success: res.GetSuccess(), msg: res.GetMessage()}
	}
}

func (m Model) lockVaultCmd() tea.Cmd {
	return func() tea.Msg {
		if m.VaultClient == nil {
			return vaultLockedMsg{success: false}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		res, err := m.VaultClient.Lock(ctx, &pb.LockRequest{})
		if err != nil {
			return vaultLockedMsg{success: false}
		}

		return vaultLockedMsg{success: res.GetSuccess()}
	}
}

func (m Model) transferFileCmd(src, dst string) tea.Cmd {
	return func() tea.Msg {
		if m.FileClient == nil {
			return transferStartedMsg{success: false, msg: "gRPC server not connected"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		res, err := m.FileClient.Transfer(ctx, &pb.TransferFileRequest{Source: src, Destination: dst})
		if err != nil {
			return transferStartedMsg{success: false, msg: err.Error()}
		}

		return transferStartedMsg{taskId: res.GetTaskId(), success: res.GetSuccess(), msg: "Transfer started"}
	}
}

func (m Model) checkProgressCmd(taskId string) tea.Cmd {
	return func() tea.Msg {
		if m.StatusClient == nil {
			return progressMsg{done: true}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		res, err := m.StatusClient.GetProgress(ctx, &pb.GetProgressRequest{TaskId: taskId})
		if err != nil {
			return progressMsg{done: true}
		}

		perc := res.GetPercentage() / 100.0
		done := res.GetPercentage() >= 100.0 || res.GetStatus() == "completed"

		return progressMsg{
			percentage: perc,
			status:     res.GetStatus(),
			done:       done,
		}
	}
}

// Update handles incoming messages and key/mouse events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.ProgressBar.Width = msg.Width - 20
		if m.ProgressBar.Width < 10 {
			m.ProgressBar.Width = 10
		}

	case grpcConnectedMsg:
		if msg.err != nil {
			m.GrpcConnected = false
			m.StatusMsg = "gRPC Connection Failed (:50051)"
		} else {
			m.GrpcConnected = true
			m.GrpcConn = msg.conn
			m.VaultClient = msg.vaultClient
			m.FileClient = msg.fileClient
			m.StatusClient = msg.statusClient
			m.StatusMsg = "gRPC Connected (:50051)"
			cmds = append(cmds, m.listRemoteFilesCmd(m.RemotePane.CurrentPath))
		}

	case remoteFilesMsg:
		if msg.err == nil && len(msg.files) > 0 {
			m.RemotePane.Files = msg.files
			if m.RemotePane.Cursor >= len(msg.files) {
				m.RemotePane.Cursor = 0
			}
		}

	case vaultUnlockedMsg:
		if msg.success {
			m.VaultUnlocked = true
			m.StatusMsg = "🔓 Vault Unlocked Successfully"
		} else {
			m.VaultUnlocked = false
			m.StatusMsg = "Vault Unlock Failed: " + msg.msg
		}
		m.ShowingVaultModal = false
		m.PassphraseInput.Reset()

	case vaultLockedMsg:
		m.VaultUnlocked = false
		m.StatusMsg = "🔒 Vault Locked"

	case transferStartedMsg:
		if msg.success {
			m.TransferActive = true
			m.TaskId = msg.taskId
			m.TransferProgress = 0.1
			m.StatusMsg = "Transfer Started: " + msg.taskId
			cmds = append(cmds, m.checkProgressCmd(m.TaskId))
		} else {
			m.StatusMsg = "Transfer Failed: " + msg.msg
		}

	case progressMsg:
		m.TransferProgress = msg.percentage
		if msg.done {
			m.TransferActive = false
			m.TransferProgress = 1.0
			m.StatusMsg = "Transfer Complete"
		} else {
			cmds = append(cmds, func() tea.Msg {
				time.Sleep(500 * time.Millisecond)
				return tickMsg(time.Now())
			})
		}

	case tickMsg:
		if m.TransferActive {
			cmds = append(cmds, m.checkProgressCmd(m.TaskId))
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			if msg.X < m.Width/2 {
				m.ActivePane = PaneLocal
			} else {
				m.ActivePane = PaneRemote
			}
		}

	case tea.KeyMsg:
		// Modal input takes precedence
		if m.ShowingVaultModal {
			switch msg.String() {
			case "enter":
				passphrase := m.PassphraseInput.Value()
				return m, m.unlockVaultCmd(passphrase)
			case "esc":
				m.ShowingVaultModal = false
				m.PassphraseInput.Reset()
				return m, nil
			default:
				var cmd tea.Cmd
				m.PassphraseInput, cmd = m.PassphraseInput.Update(msg)
				return m, cmd
			}
		}

		active := m.GetActivePane()

		switch msg.String() {
		case "ctrl+c", "q", "f10":
			if m.GrpcConn != nil {
				_ = m.GrpcConn.Close()
			}
			return m, tea.Quit

		case "tab":
			if m.ActivePane == PaneLocal {
				m.ActivePane = PaneRemote
			} else {
				m.ActivePane = PaneLocal
			}

		case "up", "k":
			if active.Cursor > 0 {
				active.Cursor--
			}

		case "down", "j":
			if active.Cursor < len(active.Files)-1 {
				active.Cursor++
			}

		case "g":
			active.Cursor = 0

		case "G":
			if len(active.Files) > 0 {
				active.Cursor = len(active.Files) - 1
			}

		case "left", "h", "backspace":
			if active.CurrentPath != "/" && active.CurrentPath != "." {
				active.CurrentPath = filepath.Dir(active.CurrentPath)
				if active.Type == PaneLocal {
					m.LoadLocalFiles()
				} else {
					cmds = append(cmds, m.listRemoteFilesCmd(active.CurrentPath))
				}
			}

		case "enter", "right", "l":
			if len(active.Files) > 0 && active.Cursor < len(active.Files) {
				item := active.Files[active.Cursor]
				if item.IsDir {
					if item.Name == ".." {
						active.CurrentPath = filepath.Dir(active.CurrentPath)
					} else {
						active.CurrentPath = item.Path
					}
					if active.Type == PaneLocal {
						m.LoadLocalFiles()
					} else {
						cmds = append(cmds, m.listRemoteFilesCmd(active.CurrentPath))
					}
				}
			}

		case "f1":
			m.StatusMsg = "F1: Total Commander TUI v0.1 | Keys: Vim (j,k,h,l,g,G) + Arrows"

		case "f2":
			if m.VaultUnlocked {
				return m, m.lockVaultCmd()
			}
			m.ShowingVaultModal = true
			m.PassphraseInput.Focus()
			return m, textinput.Blink

		case "f5":
			if len(active.Files) > 0 && active.Cursor < len(active.Files) {
				src := active.Files[active.Cursor].Path
				dst := "/destination/" + active.Files[active.Cursor].Name
				return m, m.transferFileCmd(src, dst)
			}

		case "f8":
			m.StatusMsg = "Delete requested (F8)"

		case "f9":
			return m, m.connectGrpcCmd()
		}
	}

	return m, tea.Batch(cmds...)
}
