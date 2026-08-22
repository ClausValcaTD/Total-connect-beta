package tui

import "github.com/charmbracelet/lipgloss"

// Dark mode color definitions
var (
	ColorBgDark        = lipgloss.Color("#1a1b26")
	ColorBgPane        = lipgloss.Color("#24283b")
	ColorBgActivePane  = lipgloss.Color("#1f2335")
	ColorBorderActive  = lipgloss.Color("#7aa2f7") // Blue
	ColorBorderDim     = lipgloss.Color("#414868") // Slate Gray
	ColorHeaderBg      = lipgloss.Color("#3b4261")
	ColorHeaderFg      = lipgloss.Color("#c0caf5")
	ColorDir           = lipgloss.Color("#7dcfff") // Cyan
	ColorFile          = lipgloss.Color("#9ece6a") // Green
	ColorSelectedBg    = lipgloss.Color("#364a82")
	ColorSelectedFg    = lipgloss.Color("#ffffff")
	ColorStatusSuccess = lipgloss.Color("#9ece6a")
	ColorStatusErr     = lipgloss.Color("#f7768e")
	ColorFuncKeyNum    = lipgloss.Color("#bb9af7")
	ColorFuncKeyLabel  = lipgloss.Color("#a9b1d6")
	ColorFuncBarBg     = lipgloss.Color("#16161e")
)

// Styles holds all Lipgloss styles for the dark theme Total Commander TUI.
type Styles struct {
	Header           lipgloss.Style
	StatusConnected  lipgloss.Style
	StatusDisconnect lipgloss.Style
	VaultLocked      lipgloss.Style
	VaultUnlocked    lipgloss.Style
	ActivePane       lipgloss.Style
	InactivePane     lipgloss.Style
	PaneTitleActive  lipgloss.Style
	PaneTitleDim     lipgloss.Style
	FileDir          lipgloss.Style
	FileItem         lipgloss.Style
	FileSelected     lipgloss.Style
	FuncKeyNum       lipgloss.Style
	FuncKeyLabel     lipgloss.Style
	FuncBar          lipgloss.Style
	DialogBox        lipgloss.Style
}

// DefaultStyles returns standard Lipgloss styles for the dark theme TUI.
func DefaultStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Background(ColorHeaderBg).
			Foreground(ColorHeaderFg).
			Padding(0, 1),

		StatusConnected: lipgloss.NewStyle().
			Foreground(ColorStatusSuccess).
			Bold(true),

		StatusDisconnect: lipgloss.NewStyle().
			Foreground(ColorStatusErr).
			Bold(true),

		VaultLocked: lipgloss.NewStyle().
			Foreground(ColorStatusErr).
			Bold(true),

		VaultUnlocked: lipgloss.NewStyle().
			Foreground(ColorStatusSuccess).
			Bold(true),

		ActivePane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderActive).
			Background(ColorBgActivePane).
			Padding(0, 1),

		InactivePane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderDim).
			Background(ColorBgPane).
			Padding(0, 1),

		PaneTitleActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBorderActive),

		PaneTitleDim: lipgloss.NewStyle().
			Foreground(ColorBorderDim),

		FileDir: lipgloss.NewStyle().
			Foreground(ColorDir).
			Bold(true),

		FileItem: lipgloss.NewStyle().
			Foreground(ColorFile),

		FileSelected: lipgloss.NewStyle().
			Background(ColorSelectedBg).
			Foreground(ColorSelectedFg).
			Bold(true),

		FuncKeyNum: lipgloss.NewStyle().
			Background(ColorFuncKeyNum).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1),

		FuncKeyLabel: lipgloss.NewStyle().
			Background(ColorFuncBarBg).
			Foreground(ColorFuncKeyLabel).
			Padding(0, 1),

		FuncBar: lipgloss.NewStyle().
			Background(ColorFuncBarBg),

		DialogBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorBorderActive).
			Padding(1, 2).
			Align(lipgloss.Center),
	}
}
