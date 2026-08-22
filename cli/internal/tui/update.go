package tui

import (
	"context"
	"fmt"
	"os"
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

type mkDirResultMsg struct {
	path    string
	isLocal bool
	err     error
}

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

		conn, err := grpc.DialContext(ctx, m.GrpcAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
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

		res, err := m.FileClient.Transfer(ctx, &pb.TransferFileRequest{
			Source:      src,
			Destination: dst,
		})
		if err != nil {
			return transferStartedMsg{success: false, msg: err.Error()}
		}

		return transferStartedMsg{
			taskId:  res.GetTaskId(),
			success: res.GetSuccess(),
			msg:     "Transfer started",
		}
	}
}

func (m Model) syncFilesCmd(src, dst string) tea.Cmd {
	return func() tea.Msg {
		if m.FileClient == nil {
			return transferStartedMsg{success: false, msg: "gRPC server not connected"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		res, err := m.FileClient.Sync(ctx, &pb.SyncFilesRequest{
			Source:            src,
			Destination:       dst,
			DeleteExtraneous:  false,
		})
		if err != nil {
			return transferStartedMsg{success: false, msg: err.Error()}
		}

		return transferStartedMsg{
			taskId:  res.GetTaskId(),
			success: res.GetSuccess(),
			msg:     "Sync started",
		}
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

func (m Model) mkDirCmd(path string, isLocal bool) tea.Cmd {
	return func() tea.Msg {
		if isLocal {
			err := os.MkdirAll(path, 0o755)
			return mkDirResultMsg{path: path, isLocal: true, err: err}
		}
		// Remote mkdir via gRPC — reuse Transfer with isDir hint via path ending in /
		if m.FileClient == nil {
			return mkDirResultMsg{err: grpc.ErrServerStopped, isLocal: false}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// proto Delete/Transfer don't have mkdir — use Sync with empty source as workaround
		// until proto adds a Mkdir RPC; for now only local mkdir is fully supported
		_ = ctx
		_ = cancel
		return mkDirResultMsg{path: path, isLocal: false, err: fmt.Errorf("remote mkdir requires server-side support")}
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
			m.StatusMsg = "gRPC Connection Failed — press F9 to retry"
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
		if msg.err != nil {
			// FIX #5: show helpful message when remote is unreachable
			m.RemotePane.Files = []FileItem{
				{Name: "[ Not connected — press F9 to reconnect ]", IsDir: false},
			}
		} else {
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
			m.StatusMsg = "✅ Transfer Complete"
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

	case mkDirResultMsg:
		if msg.err != nil {
			m.StatusMsg = "❌ MkDir failed: " + msg.err.Error()
		} else {
			m.StatusMsg = "✅ Created: " + msg.path
			if msg.isLocal {
				m.LoadLocalFiles()
			} else {
				cmds = append(cmds, m.listRemoteFilesCmd(m.RemotePane.CurrentPath))
			}
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
		// FIX #7: Copy confirmation dialog takes top priority
		if m.ShowingCopyDialog {
			switch msg.String() {
			case "enter", "y", "Y":
				m.ShowingCopyDialog = false
				return m, m.transferFileCmd(m.CopyDialogSrc, m.CopyDialogDst)
			case "esc", "n", "N":
				m.ShowingCopyDialog = false
				m.StatusMsg = "Copy cancelled"
			}
			return m, nil
		}

		// MkDir modal
		if m.ShowingMkDirModal {
			switch msg.String() {
			case "enter":
				name := m.MkDirInput.Value()
				if name == "" {
					m.ShowingMkDirModal = false
					break
				}
				active := m.GetActivePane()
				newPath := filepath.Join(active.CurrentPath, name)
				isLocal := m.ActivePane == PaneLocal
				m.ShowingMkDirModal = false
				m.MkDirInput.Reset()
				return m, m.mkDirCmd(newPath, isLocal)
			case "esc":
				m.ShowingMkDirModal = false
				m.MkDirInput.Reset()
				return m, nil
			default:
				var cmd tea.Cmd
				m.MkDirInput, cmd = m.MkDirInput.Update(msg)
				return m, cmd
			}
		}

		// Profiles modal input
		if m.ShowingProfilesModal {
			switch msg.String() {
			case "down", "j":
				if m.ProfileCursor < len(m.Profiles)-1 {
					m.ProfileCursor++
				}
				return m, nil
			case "up", "k":
				if m.ProfileCursor > 0 {
					m.ProfileCursor--
				}
				return m, nil
			case "enter":
				if len(m.Profiles) > 0 && m.ProfileCursor >= 0 && m.ProfileCursor < len(m.Profiles) {
					p := m.Profiles[m.ProfileCursor]
					m.GrpcAddr = fmt.Sprintf("%s:%d", p.Host, p.Port)
					if p.Path != "" {
						m.RemotePane.CurrentPath = p.Path
					}
					m.ShowingProfilesModal = false
					return m, m.connectGrpcCmd()
				}
				m.ShowingProfilesModal = false
				return m, nil
			case "n":
				m.StatusMsg = "New profile: coming soon"
				return m, nil
			case "delete":
				if len(m.Profiles) > 0 && m.ProfileCursor >= 0 && m.ProfileCursor < len(m.Profiles) {
					m.Profiles = append(m.Profiles[:m.ProfileCursor], m.Profiles[m.ProfileCursor+1:]...)
					if m.ProfileCursor >= len(m.Profiles) && m.ProfileCursor > 0 {
						m.ProfileCursor--
					}
					_ = SaveProfiles(m.Profiles, "")
				}
				return m, nil
			case "esc":
				m.ShowingProfilesModal = false
				return m, nil
			default:
				return m, nil
			}
		}

		// Vault modal input
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

		// FIX #4: work directly on LocalPane/RemotePane fields,
		// not via a local pointer that doesn't write back to Model.
		switch msg.String() {
		case "ctrl+c", "f10":
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
			if m.ActivePane == PaneLocal {
				if m.LocalPane.Cursor > 0 {
					m.LocalPane.Cursor--
				}
			} else {
				if m.RemotePane.Cursor > 0 {
					m.RemotePane.Cursor--
				}
			}

		case "down", "j":
			if m.ActivePane == PaneLocal {
				if m.LocalPane.Cursor < len(m.LocalPane.Files)-1 {
					m.LocalPane.Cursor++
				}
			} else {
				if m.RemotePane.Cursor < len(m.RemotePane.Files)-1 {
					m.RemotePane.Cursor++
				}
			}

		case "g":
			if m.ActivePane == PaneLocal {
				m.LocalPane.Cursor = 0
			} else {
				m.RemotePane.Cursor = 0
			}

		case "G":
			if m.ActivePane == PaneLocal {
				if len(m.LocalPane.Files) > 0 {
					m.LocalPane.Cursor = len(m.LocalPane.Files) - 1
				}
			} else {
				if len(m.RemotePane.Files) > 0 {
					m.RemotePane.Cursor = len(m.RemotePane.Files) - 1
				}
			}

		case "left", "h", "backspace":
			if m.ActivePane == PaneLocal {
				if m.LocalPane.CurrentPath != "/" && m.LocalPane.CurrentPath != "." {
					m.LocalPane.CurrentPath = filepath.Dir(m.LocalPane.CurrentPath)
					m.LocalPane.Cursor = 0
					m.LoadLocalFiles()
				}
			} else {
				if m.RemotePane.CurrentPath != "/" && m.RemotePane.CurrentPath != "" {
					m.RemotePane.CurrentPath = filepath.Dir(m.RemotePane.CurrentPath)
					m.RemotePane.Cursor = 0
					cmds = append(cmds, m.listRemoteFilesCmd(m.RemotePane.CurrentPath))
				}
			}

		case "enter", "right", "l":
			if m.ActivePane == PaneLocal {
				if len(m.LocalPane.Files) > 0 && m.LocalPane.Cursor < len(m.LocalPane.Files) {
					item := m.LocalPane.Files[m.LocalPane.Cursor]
					if item.IsDir {
						if item.Name == ".." {
							m.LocalPane.CurrentPath = filepath.Dir(m.LocalPane.CurrentPath)
						} else {
							m.LocalPane.CurrentPath = item.Path
						}
						m.LocalPane.Cursor = 0
						m.LoadLocalFiles()
					}
				}
			} else {
				if len(m.RemotePane.Files) > 0 && m.RemotePane.Cursor < len(m.RemotePane.Files) {
					item := m.RemotePane.Files[m.RemotePane.Cursor]
					if item.IsDir {
						if item.Name == ".." {
							m.RemotePane.CurrentPath = filepath.Dir(m.RemotePane.CurrentPath)
						} else {
							m.RemotePane.CurrentPath = item.Path
						}
						m.RemotePane.Cursor = 0
						cmds = append(cmds, m.listRemoteFilesCmd(m.RemotePane.CurrentPath))
					}
				}
			}

		case "f1":
			m.StatusMsg = "F1: Total Connect TUI | Keys: j/k up/down  h/l nav  Tab switch  F2 vault  F5 copy  F4 sync  F9 reconnect  F10 quit"

		case "f2":
			if m.VaultUnlocked {
				return m, m.lockVaultCmd()
			}
			m.ShowingVaultModal = true
			m.PassphraseInput.Focus()
			return m, textinput.Blink

		case "f3":
			m.ShowingProfilesModal = true
			m.ProfileCursor = 0
			return m, nil

		case "f4":
			// FIX #10: F4 Sync — sync active pane to inactive pane
			active := m.GetActivePane()
			inactive := m.GetInactivePane()
			if len(active.Files) == 0 {
				m.StatusMsg = "Nothing to sync"
				break
			}
			src := active.CurrentPath
			dst := inactive.CurrentPath
			m.StatusMsg = "Syncing " + src + " → " + dst
			return m, m.syncFilesCmd(src, dst)

		case " ":
			var pane *Pane
			if m.ActivePane == PaneLocal {
				pane = &m.LocalPane
			} else {
				pane = &m.RemotePane
			}
			if pane.Selected == nil {
				pane.Selected = make(map[string]bool)
			}
			if len(pane.Files) > 0 && pane.Cursor < len(pane.Files) {
				item := pane.Files[pane.Cursor]
				if item.Name != ".." {
					if pane.Selected[item.Path] {
						delete(pane.Selected, item.Path)
					} else {
						pane.Selected[item.Path] = true
					}
				}
				if pane.Cursor < len(pane.Files)-1 {
					pane.Cursor++
				}
			}

		case "esc":
			if m.ActivePane == PaneLocal {
				m.LocalPane.Selected = make(map[string]bool)
			} else {
				m.RemotePane.Selected = make(map[string]bool)
			}

		case "f5":
			active := m.GetActivePane()
			inactive := m.GetInactivePane()

			var selectedPaths []string
			for path, sel := range active.Selected {
				if sel {
					selectedPaths = append(selectedPaths, path)
				}
			}

			if len(selectedPaths) > 0 {
				for _, src := range selectedPaths {
					dst := filepath.Join(inactive.CurrentPath, filepath.Base(src))
					cmds = append(cmds, m.transferFileCmd(src, dst))
				}
				return m, tea.Batch(cmds...)
			} else {
				// Existing single-file behavior
				if len(active.Files) == 0 || active.Cursor >= len(active.Files) {
					break
				}
				item := active.Files[active.Cursor]
				if item.Name == ".." {
					break
				}
				src := item.Path
				dst := filepath.Join(inactive.CurrentPath, item.Name)
				m.ShowingCopyDialog = true
				m.CopyDialogSrc = src
				m.CopyDialogDst = dst
			}

		case "f7":
			m.ShowingMkDirModal = true
			m.MkDirInput.Reset()
			m.MkDirInput.Focus()
			return m, textinput.Blink

		case "f8":
			m.StatusMsg = "Delete: F8 — not yet implemented"

		case "f9":
			m.GrpcConnected = false
			m.StatusMsg = "Reconnecting..."
			return m, m.connectGrpcCmd()
		}
	}

	return m, tea.Batch(cmds...)
}
