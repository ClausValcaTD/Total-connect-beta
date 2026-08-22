package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/totalconnect/cli/internal/tui"
)

func TestInitialModel(t *testing.T) {
	m := tui.InitialModel()

	if m.ActivePane != tui.PaneLocal {
		t.Errorf("expected initial active pane to be PaneLocal (0), got %v", m.ActivePane)
	}

	if len(m.LocalPane.Files) == 0 {
		t.Errorf("expected local pane to load files from current directory")
	}

	if m.GrpcAddr != ":50051" {
		t.Errorf("expected gRPC address to default to :50051, got %s", m.GrpcAddr)
	}
}

func TestKeyNavigationAndPaneSwitching(t *testing.T) {
	m := tui.InitialModel()

	// Initial cursor = 0
	if m.LocalPane.Cursor != 0 {
		t.Fatalf("expected cursor = 0, got %d", m.LocalPane.Cursor)
	}

	// Send 'j' (down) key
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updatedModel.(tui.Model)

	if len(m.LocalPane.Files) > 1 && m.LocalPane.Cursor != 1 {
		t.Errorf("expected cursor = 1 after 'j', got %d", m.LocalPane.Cursor)
	}

	// Send 'k' (up) key
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updatedModel.(tui.Model)

	if m.LocalPane.Cursor != 0 {
		t.Errorf("expected cursor = 0 after 'k', got %d", m.LocalPane.Cursor)
	}

	// Send 'tab' key to switch pane
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updatedModel.(tui.Model)

	if m.ActivePane != tui.PaneRemote {
		t.Errorf("expected active pane to be PaneRemote after tab, got %v", m.ActivePane)
	}

	// Send 'tab' key again to return to Local
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updatedModel.(tui.Model)

	if m.ActivePane != tui.PaneLocal {
		t.Errorf("expected active pane to return to PaneLocal after second tab, got %v", m.ActivePane)
	}
}

func TestMousePaneSelection(t *testing.T) {
	m := tui.InitialModel()
	m.Width = 100
	m.Height = 30

	// Mouse press on left half (X = 20) -> PaneLocal
	updatedModel, _ := m.Update(tea.MouseMsg{X: 20, Y: 10, Action: tea.MouseActionPress})
	m = updatedModel.(tui.Model)
	if m.ActivePane != tui.PaneLocal {
		t.Errorf("expected PaneLocal when clicking on left half")
	}

	// Mouse press on right half (X = 80) -> PaneRemote
	updatedModel, _ = m.Update(tea.MouseMsg{X: 80, Y: 10, Action: tea.MouseActionPress})
	m = updatedModel.(tui.Model)
	if m.ActivePane != tui.PaneRemote {
		t.Errorf("expected PaneRemote when clicking on right half")
	}
}

func TestViewRendering(t *testing.T) {
	m := tui.InitialModel()

	// Initial View before window size set
	viewPre := m.View()
	if !strings.Contains(viewPre, "Initializing") {
		t.Errorf("expected initial message before window size msg")
	}

	// Send WindowSizeMsg
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updatedModel.(tui.Model)

	viewStr := m.View()
	if !strings.Contains(viewStr, "Total Connect") {
		t.Errorf("expected header in view output")
	}
	if !strings.Contains(viewStr, "Local") || !strings.Contains(viewStr, "Remote") {
		t.Errorf("expected local and remote pane titles in view output")
	}
	if !strings.Contains(viewStr, "F10") {
		t.Errorf("expected F1-F10 function bar in view output")
	}
}

func TestMultiSelectSpaceAndEsc(t *testing.T) {
	m := tui.InitialModel()
	m.LocalPane.Files = []tui.FileItem{
		{Name: "..", Path: "/home", IsDir: true},
		{Name: "file1.txt", Path: "/home/user/file1.txt", IsDir: false},
		{Name: "file2.txt", Path: "/home/user/file2.txt", IsDir: false},
	}
	m.LocalPane.Cursor = 1

	// Toggle selected file1.txt
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updatedModel.(tui.Model)

	if !m.LocalPane.Selected["/home/user/file1.txt"] {
		t.Errorf("expected file1.txt to be selected")
	}
	if m.LocalPane.Cursor != 2 {
		t.Errorf("expected cursor to move to 2, got %d", m.LocalPane.Cursor)
	}

	// Render view and check for selected count
	m.Width = 100
	m.Height = 30
	viewStr := m.View()
	if !strings.Contains(viewStr, "1 selected") {
		t.Errorf("expected title to show selected count")
	}

	// Press Esc to clear selection
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(tui.Model)

	if len(m.LocalPane.Selected) != 0 {
		t.Errorf("expected Selected map to be cleared after Esc")
	}
}

func TestProfilesLoadSaveModal(t *testing.T) {
	plain := "secret123"
	pass := "myvaultpass"

	enc, err := tui.ExportEncryptPassword(plain, pass)
	if err != nil {
		t.Fatalf("encrypt password failed: %v", err)
	}

	dec, err := tui.ExportDecryptPassword(enc, pass)
	if err != nil {
		t.Fatalf("decrypt password failed: %v", err)
	}

	if dec != plain {
		t.Errorf("expected decrypted password %s, got %s", plain, dec)
	}

	m := tui.InitialModel()
	m.Width = 100
	m.Height = 30

	// Open profiles modal with F3
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyF3})
	m = updatedModel.(tui.Model)

	if !m.ShowingProfilesModal {
		t.Errorf("expected profiles modal to be visible after F3")
	}

	viewStr := m.View()
	if !strings.Contains(viewStr, "Connection Profiles") {
		t.Errorf("expected modal title in view output")
	}

	// Press Esc to close
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(tui.Model)

	if m.ShowingProfilesModal {
		t.Errorf("expected profiles modal to close after Esc")
	}
}

func TestVaultModalToggle(t *testing.T) {
	m := tui.InitialModel()
	m.Width = 100
	m.Height = 30

	if m.ShowingVaultModal {
		t.Fatalf("vault modal should be closed initially")
	}

	// Press F2 to open vault modal
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	m = updatedModel.(tui.Model)

	if !m.ShowingVaultModal {
		t.Errorf("expected Vault modal to open after pressing F2")
	}

	viewStr := m.View()
	if !strings.Contains(viewStr, "Unlock Vault") {
		t.Errorf("expected modal content in view rendering")
	}

	// Press Esc to cancel
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(tui.Model)

	if m.ShowingVaultModal {
		t.Errorf("expected Vault modal to close after pressing Esc")
	}
}
