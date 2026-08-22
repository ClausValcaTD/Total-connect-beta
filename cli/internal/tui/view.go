package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI layout.
func (m Model) View() string {
	if m.Width == 0 {
		return "Initializing Total Commander TUI..."
	}

	// 1. Header Bar
	var connStatus string
	if m.GrpcConnected {
		connStatus = m.Styles.StatusConnected.Render("● Connected (:50051)")
	} else {
		connStatus = m.Styles.StatusDisconnect.Render("○ Offline (:50051)")
	}

	var vaultStatus string
	if m.VaultUnlocked {
		vaultStatus = m.Styles.VaultUnlocked.Render("🔓 Vault Unlocked")
	} else {
		vaultStatus = m.Styles.VaultLocked.Render("🔒 Vault Locked")
	}

	headerContent := fmt.Sprintf(" Total Commander TUI   |   %s   |   %s", connStatus, vaultStatus)
	header := m.Styles.Header.Width(m.Width).Render(headerContent)

	// Calculate inner dimensions
	paneWidth := (m.Width / 2) - 2
	if paneWidth < 20 {
		paneWidth = 20
	}
	paneHeight := m.Height - 7
	if paneHeight < 5 {
		paneHeight = 5
	}

	// 2. Local & Remote Panes
	localView := m.renderPane(&m.LocalPane, m.ActivePane == PaneLocal, paneWidth, paneHeight)
	remoteView := m.renderPane(&m.RemotePane, m.ActivePane == PaneRemote, paneWidth, paneHeight)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, localView, remoteView)

	// 3. Progress Bar or Status Bar
	var middleBar string
	if m.TransferActive {
		barView := m.ProgressBar.ViewAs(m.TransferProgress)
		middleBar = fmt.Sprintf(" Transferring: [%s] %.0f%%  %s", barView, m.TransferProgress*100, m.StatusMsg)
	} else {
		middleBar = fmt.Sprintf(" Status: %s", m.StatusMsg)
	}
	statusLine := lipgloss.NewStyle().
		Width(m.Width).
		Background(lipgloss.Color("#24283b")).
		Foreground(lipgloss.Color("#a9b1d6")).
		Render(middleBar)

	// 4. Function Keys Bar (F1-F10)
	funcBar := m.renderFuncBar()

	// 5. Combine layout
	fullView := lipgloss.JoinVertical(lipgloss.Left, header, panes, statusLine, funcBar)

	// Overlay modal if passphrase input requested
	if m.ShowingVaultModal {
		return m.renderVaultModal(fullView)
	}

	return fullView
}

func (m Model) renderPane(pane *Pane, isActive bool, width, height int) string {
	style := m.Styles.InactivePane
	titleStyle := m.Styles.PaneTitleDim
	if isActive {
		style = m.Styles.ActivePane
		titleStyle = m.Styles.PaneTitleActive
	}

	typeStr := "Local"
	if pane.Type == PaneRemote {
		typeStr = "Remote"
	}

	title := titleStyle.Render(fmt.Sprintf("[%s] %s", typeStr, pane.CurrentPath))

	var lines []string
	lines = append(lines, title, "")

	maxItems := height - 3
	if maxItems < 1 {
		maxItems = 1
	}

	startIdx := 0
	if pane.Cursor >= maxItems {
		startIdx = pane.Cursor - maxItems + 1
	}

	endIdx := startIdx + maxItems
	if endIdx > len(pane.Files) {
		endIdx = len(pane.Files)
	}

	for i := startIdx; i < endIdx; i++ {
		item := pane.Files[i]
		icon := "📄"
		itemStyle := m.Styles.FileItem
		if item.IsDir {
			icon = "📁"
			itemStyle = m.Styles.FileDir
		}

		sizeStr := fmt.Sprintf("%8d B", item.Size)
		if item.IsDir {
			sizeStr = "    <DIR> "
		}

		nameTrunc := item.Name
		maxNameLen := width - 20
		if maxNameLen > 5 && len(nameTrunc) > maxNameLen {
			nameTrunc = nameTrunc[:maxNameLen-3] + "..."
		}

		lineStr := fmt.Sprintf("%s %-20s %s", icon, nameTrunc, sizeStr)

		if i == pane.Cursor {
			if isActive {
				lineStr = m.Styles.FileSelected.Render(lineStr)
			} else {
				lineStr = lipgloss.NewStyle().Background(lipgloss.Color("#3b4261")).Render(lineStr)
			}
		} else {
			lineStr = itemStyle.Render(lineStr)
		}

		lines = append(lines, lineStr)
	}

	// Fill remaining height with empty lines
	for len(lines) < height {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return style.Width(width).Height(height).Render(content)
}

func (m Model) renderFuncBar() string {
	funcs := []struct {
		Key   string
		Label string
	}{
		{"F1", "Help"},
		{"F2", "Unl/Lck"},
		{"F3", "View"},
		{"F4", "Sync"},
		{"F5", "Copy"},
		{"F6", "Move"},
		{"F7", "MkDir"},
		{"F8", "Del"},
		{"F9", "Reconn"},
		{"F10", "Quit"},
	}

	var items []string
	for _, f := range funcs {
		num := m.Styles.FuncKeyNum.Render(f.Key)
		lbl := m.Styles.FuncKeyLabel.Render(f.Label)
		items = append(items, num+lbl)
	}

	barContent := strings.Join(items, " ")
	return m.Styles.FuncBar.Width(m.Width).Render(barContent)
}

func (m Model) renderVaultModal(background string) string {
	modalContent := fmt.Sprintf(
		"🔐 Unlock Vault\n\nEnter Master Passphrase:\n\n%s\n\n[Enter] Confirm  |  [Esc] Cancel",
		m.PassphraseInput.View(),
	)

	dialog := m.Styles.DialogBox.Render(modalContent)

	// Center dialog over screen
	return lipgloss.Place(
		m.Width,
		m.Height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
	)
}
