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
	if !strings.Contains(viewStr, "Total Commander TUI") {
		t.Errorf("expected header in view output")
	}
	if !strings.Contains(viewStr, "Local") || !strings.Contains(viewStr, "Remote") {
		t.Errorf("expected local and remote pane titles in view output")
	}
	if !strings.Contains(viewStr, "F10") {
		t.Errorf("expected F1-F10 function bar in view output")
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

func TestMultiSelectAndEscClear(t *testing.T) {
	m := tui.InitialModel()
	m.Width = 100
	m.Height = 30

	if len(m.LocalPane.Files) == 0 {
		t.Fatalf("expected local pane files")
	}

	firstPath := m.LocalPane.Files[0].Path

	// Press space to select first item
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updatedModel.(tui.Model)

	if !m.LocalPane.Selected[firstPath] {
		t.Errorf("expected first file to be selected after pressing Space")
	}

	// View should show (1 selected)
	viewStr := m.View()
	if !strings.Contains(viewStr, "(1 selected)") {
		t.Errorf("expected View to contain '(1 selected)', got: %s", viewStr)
	}

	// Press Esc to clear selection
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(tui.Model)

	if len(m.LocalPane.Selected) != 0 {
		t.Errorf("expected selection to be empty after pressing Esc, got %v", m.LocalPane.Selected)
	}
}

func TestProfilesModalToggle(t *testing.T) {
	m := tui.InitialModel()
	m.Width = 100
	m.Height = 30

	if m.ShowingProfilesModal {
		t.Fatalf("profiles modal should be closed initially")
	}

	// Press F3 to open profiles modal
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyF3})
	m = updatedModel.(tui.Model)

	if !m.ShowingProfilesModal {
		t.Errorf("expected Profiles modal to open after pressing F3")
	}

	viewStr := m.View()
	if !strings.Contains(viewStr, "Connection Profiles") {
		t.Errorf("expected Profiles modal content in view rendering")
	}

	// Press Esc to close
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(tui.Model)

	if m.ShowingProfilesModal {
		t.Errorf("expected Profiles modal to close after pressing Esc")
	}
}

func TestProfilesSaveAndLoad(t *testing.T) {
	profs := []tui.Profile{
		{
			Name:     "Test Server",
			Backend:  "sftp",
			Host:     "example.com",
			Port:     22,
			User:     "user",
			Password: "secretpassword",
			Path:     "/home/user",
		},
	}

	passphrase := "myvaultpass"
	err := tui.SaveProfiles(profs, passphrase)
	if err != nil {
		t.Fatalf("failed to save profiles: %v", err)
	}

	loaded, err := tui.LoadProfiles()
	if err != nil {
		t.Fatalf("failed to load profiles: %v", err)
	}

	if len(loaded) == 0 {
		t.Fatalf("expected loaded profiles not to be empty")
	}

	found := false
	for _, p := range loaded {
		if p.Name == "Test Server" {
			found = true
			if p.Backend != "sftp" || p.Host != "example.com" {
				t.Errorf("profile fields mismatch: %+v", p)
			}
		}
	}
	if !found {
		t.Errorf("expected to find saved profile in loaded profiles")
	}
}
