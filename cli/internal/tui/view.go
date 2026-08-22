package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI layout.
func (m Model) View() string {
	if m.Width == 0 {
		return "Initializing Total Connect TUI..."
	}

	// 1. Header Bar
	var connStatus string
	if m.GrpcConnected {
		connStatus = m.Styles.StatusConnected.Render("● Connected (" + m.GrpcAddr + ")")
	} else {
		connStatus = m.Styles.StatusDisconnect.Render("○ Offline (" + m.GrpcAddr + ")")
	}

	var vaultStatus string
	if m.VaultUnlocked {
		vaultStatus = m.Styles.VaultUnlocked.Render("🔓 Vault Unlocked")
	} else {
		vaultStatus = m.Styles.VaultLocked.Render("🔒 Vault Locked")
	}

	headerContent := fmt.Sprintf(" Total Commander TUI  |  %s  |  %s", connStatus, vaultStatus)
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
		middleBar = fmt.Sprintf(" Transferring: %s %.0f%%  %s", barView, m.TransferProgress*100, m.StatusMsg)
	} else {
		middleBar = fmt.Sprintf(" %s", m.StatusMsg)
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

	// Overlay modals on top of fullView
	if m.ShowingNewProfileForm {
		return m.renderNewProfileForm(fullView)
	}
	if m.ShowingProfilesModal {
		return m.renderProfilesModal(fullView)
	}
	if m.ShowingCopyDialog {
		return m.renderCopyDialog(fullView)
	}
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

	selCount := 0
	for _, sel := range pane.Selected {
		if sel {
			selCount++
		}
	}

	titleText := fmt.Sprintf("[%s %s]", typeStr, pane.CurrentPath)
	if selCount > 0 {
		titleText = fmt.Sprintf("[%s %s] (%d selected)", typeStr, pane.CurrentPath, selCount)
	}
	title := titleStyle.Render(titleText)

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

	// FIX #5: show placeholder when remote has no files yet
	if len(pane.Files) == 0 && pane.Type == PaneRemote {
		placeholder := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#414868")).
			Italic(true).
			Render("  Not connected — press F9")
		lines = append(lines, placeholder)
	}

	for i := startIdx; i < endIdx; i++ {
		item := pane.Files[i]
		icon := "📄"
		itemStyle := m.Styles.FileItem
		if item.IsDir {
			icon = "📁"
			itemStyle = m.Styles.FileDir
		}

		// FIX #6: human-readable file sizes
		var sizeStr string
		if item.IsDir {
			sizeStr = "    <DIR>"
		} else {
			sizeStr = FormatSize(item.Size)
		}

		nameTrunc := item.Name
		maxNameLen := width - 20
		if maxNameLen > 5 && len(nameTrunc) > maxNameLen {
			nameTrunc = nameTrunc[:maxNameLen-3] + "..."
		}

		isSelected := pane.Selected[item.Path]
		checkMark := " "
		if isSelected {
			checkMark = m.Styles.FileSelectedMark.Render("✓")
		}

		lineStr := fmt.Sprintf("%s %s %-*s %s", checkMark, icon, maxNameLen, nameTrunc, sizeStr)

		if i == pane.Cursor {
			if isActive {
				lineStr = m.Styles.FileSelected.Render(lineStr)
			} else {
				lineStr = lipgloss.NewStyle().
					Background(lipgloss.Color("#3b4261")).
					Render(lineStr)
			}
		} else if isSelected {
			lineStr = m.Styles.FileSelectedMark.Render(lineStr)
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
		{"F2", "Vault"},
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

// renderProfilesModal renders connection profiles list.
func (m Model) renderProfilesModal(background string) string {
	var lines []string
	lines = append(lines, "🌐 Connection Profiles", "")

	if len(m.Profiles) == 0 {
		lines = append(lines, " (No profiles saved)", "")
	} else {
		for i, p := range m.Profiles {
			cursorStr := "  "
			if i == m.ProfileCursor {
				cursorStr = "> "
			}
			hostStr := p.Host
			if p.Port > 0 {
				hostStr = fmt.Sprintf("%s:%d", p.Host, p.Port)
			}
			line := fmt.Sprintf("%s%-15s [%s] %s", cursorStr, p.Name, p.Backend, hostStr)
			if i == m.ProfileCursor {
				line = m.Styles.FileSelected.Render(line)
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	lines = append(lines, "[Enter] Connect | [n] New | [Del] Delete | [Esc] Close")

	dialog := m.Styles.DialogBox.Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
	)
}

// renderNewProfileForm renders form for creating a new profile.
func (m Model) renderNewProfileForm(background string) string {
	var lines []string
	lines = append(lines, "➕ New Connection Profile", "")

	labels := []string{"Name:", "Backend:", "Host:", "Port:", "User:", "Password:", "Path:"}
	for i, inp := range m.NewProfileInputs {
		line := fmt.Sprintf("%-10s %s", labels[i], inp.View())
		lines = append(lines, line)
	}

	lines = append(lines, "", "[Tab/Up/Down] Navigate | [Enter] Save | [Esc] Cancel")

	dialog := m.Styles.DialogBox.Render(strings.Join(lines, "\n"))

	return lipgloss.Place(
		m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
	)
}

// renderVaultModal overlays the passphrase dialog on the background.
func (m Model) renderVaultModal(background string) string {
	modalContent := fmt.Sprintf(
		"🔐 Unlock Vault\n\nEnter Master Passphrase:\n\n%s\n\n[Enter] Confirm  |  [Esc] Cancel",
		m.PassphraseInput.View(),
	)

	dialog := m.Styles.DialogBox.Render(modalContent)

	return lipgloss.Place(
		m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
	)
}

// FIX #7: renderCopyDialog shows src→dst confirmation before any transfer.
func (m Model) renderCopyDialog(background string) string {
	src := m.CopyDialogSrc
	dst := m.CopyDialogDst

	// Truncate long paths for display
	maxLen := m.Width/2 - 6
	if maxLen > 0 && len(src) > maxLen {
		src = "..." + src[len(src)-maxLen:]
	}
	if maxLen > 0 && len(dst) > maxLen {
		dst = "..." + dst[len(dst)-maxLen:]
	}

	content := fmt.Sprintf(
		"📋 Copy File\n\n  From: %s\n  To:   %s\n\n[Enter / Y] Confirm  |  [Esc / N] Cancel",
		src, dst,
	)

	dialog := m.Styles.DialogBox.Render(content)

	return lipgloss.Place(
		m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
	)
}
