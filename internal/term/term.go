package term

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SupportsProgressBar tries to determine whether the current terminal supports
// progress bars by looking into environment variables.
func SupportsProgressBar() bool {
	termProg := os.Getenv("TERM_PROGRAM")
	_, isWindowsTerminal := os.LookupEnv("WT_SESSION")

	return isWindowsTerminal || strings.Contains(strings.ToLower(termProg), "ghostty")
}

// TerminalType represents different terminal emulator types
type TerminalType string

const (
	TerminalVSCode     TerminalType = "vscode"
	TerminalCursor     TerminalType = "cursor"
	TerminalWindsurf   TerminalType = "windsurf"
	TerminalGhostty    TerminalType = "ghostty"
	TerminalWezTerm    TerminalType = "wezterm"
	TerminalITerm2     TerminalType = "iterm2"
	TerminalApple      TerminalType = "apple_terminal"
	TerminalKitty      TerminalType = "kitty"
	TerminalAlacritty  TerminalType = "alacritty"
	TerminalUnknown    TerminalType = "unknown"
)

// DetectTerminal attempts to detect the current terminal emulator
func DetectTerminal() TerminalType {
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	terminalEmulator := strings.ToLower(os.Getenv("TERMINAL_EMULATOR"))

	// Check for VSCode/Cursor/Windsurf
	if strings.Contains(termProgram, "vscode") {
		return TerminalVSCode
	}
	if strings.Contains(termProgram, "cursor") {
		return TerminalCursor
	}
	if strings.Contains(termProgram, "windsurf") {
		return TerminalWindsurf
	}

	// Check for iTerm2
	if strings.Contains(termProgram, "iterm") {
		return TerminalITerm2
	}

	// Check for Apple Terminal
	if termProgram == "apple_terminal" {
		return TerminalApple
	}

	// Check for Ghostty
	if strings.Contains(termProgram, "ghostty") || strings.Contains(terminalEmulator, "ghostty") {
		return TerminalGhostty
	}

	// Check for WezTerm
	if strings.Contains(strings.ToLower(os.Getenv("TERM")), "wezterm") ||
	   os.Getenv("WEZTERM_EXECUTABLE") != "" {
		return TerminalWezTerm
	}

	// Check for Kitty
	if strings.Contains(strings.ToLower(os.Getenv("TERM")), "kitty") ||
	   os.Getenv("KITTY_WINDOW_ID") != "" {
		return TerminalKitty
	}

	// Check for Alacritty
	if strings.Contains(terminalEmulator, "alacritty") ||
	   os.Getenv("ALACRITTY_SOCKET") != "" {
		return TerminalAlacritty
	}

	return TerminalUnknown
}

// GetTerminalSetupInstructions returns setup instructions for the detected terminal
func GetTerminalSetupInstructions() string {
	terminal := DetectTerminal()
	homeDir, _ := os.UserHomeDir()

	switch terminal {
	case TerminalVSCode:
		configPath := filepath.Join(homeDir, ".config", "Code", "User", "keybindings.json")
		return fmt.Sprintf(`# VSCode Terminal Setup

Add this to your keybindings file:
%s

{
  "key": "shift+enter",
  "command": "workbench.action.terminal.sendSequence",
  "args": {
    "text": "\\r\n"
  },
  "when": "terminalFocus"
}

After saving, restart VSCode. Shift+Enter will now insert newlines in Crush.`, configPath)

	case TerminalCursor:
		configPath := filepath.Join(homeDir, ".config", "Cursor", "User", "keybindings.json")
		return fmt.Sprintf(`# Cursor Terminal Setup

Add this to your keybindings file:
%s

{
  "key": "shift+enter",
  "command": "workbench.action.terminal.sendSequence",
  "args": {
    "text": "\\r\n"
  },
  "when": "terminalFocus"
}

After saving, restart Cursor. Shift+Enter will now insert newlines in Crush.`, configPath)

	case TerminalWindsurf:
		configPath := filepath.Join(homeDir, ".config", "Windsurf", "User", "keybindings.json")
		return fmt.Sprintf(`# Windsurf Terminal Setup

Add this to your keybindings file:
%s

{
  "key": "shift+enter",
  "command": "workbench.action.terminal.sendSequence",
  "args": {
    "text": "\\r\n"
  },
  "when": "terminalFocus"
}

After saving, restart Windsurf. Shift+Enter will now insert newlines in Crush.`, configPath)

	case TerminalGhostty:
		configPath := filepath.Join(homeDir, ".config", "ghostty", "config")
		return fmt.Sprintf(`# Ghostty Terminal Setup

Add this line to your config file:
%s

keybind = shift+enter=text:\x1b\r

After saving, restart Ghostty. Shift+Enter will now insert newlines in Crush.`, configPath)

	case TerminalWezTerm:
		configPath := filepath.Join(homeDir, ".wezterm.lua")
		return fmt.Sprintf(`# WezTerm Terminal Setup

Add this to your config file:
%s

config.keys = config.keys or {}
table.insert(config.keys, {
  key = "Enter",
  mods = "SHIFT",
  action = wezterm.action{SendString="\\x1b\\r"}
})

After saving, restart WezTerm. Shift+Enter will now insert newlines in Crush.`, configPath)

	case TerminalITerm2:
		return `# iTerm2 Terminal Setup

1. Open iTerm2 Preferences (⌘,)
2. Go to Profiles > Keys > Key Mappings
3. Click the "+" button to add a new key mapping
4. Set:
   - Keyboard Shortcut: ⇧↩ (Shift+Enter)
   - Action: Send Escape Sequence
   - Esc+: \r

After applying, Shift+Enter will insert newlines in Crush.`

	case TerminalApple:
		return `# Apple Terminal Setup

Apple Terminal doesn't support custom Shift+Enter bindings.

Alternative options:
1. Use Option+Enter instead (already works in Crush)
2. Use Ctrl+J for newlines (already works in Crush)
3. Switch to a more feature-rich terminal like iTerm2, Ghostty, or WezTerm`

	case TerminalKitty:
		configPath := filepath.Join(homeDir, ".config", "kitty", "kitty.conf")
		return fmt.Sprintf(`# Kitty Terminal Setup

Add this line to your config file:
%s

map shift+enter send_text all \x1b\r

After saving, restart Kitty. Shift+Enter will now insert newlines in Crush.`, configPath)

	case TerminalAlacritty:
		configPath := filepath.Join(homeDir, ".config", "alacritty", "alacritty.toml")
		return fmt.Sprintf(`# Alacritty Terminal Setup

Add this to your config file:
%s

[[keyboard.bindings]]
key = "Return"
mods = "Shift"
chars = "\x1b\r"

After saving, restart Alacritty. Shift+Enter will now insert newlines in Crush.`, configPath)

	default:
		return `# Terminal Setup - Unknown Terminal

Could not detect your terminal type.

Crush supports Shift+Enter for newlines when your terminal is configured to send
the escape sequence \x1b\r (ESC + Carriage Return) for Shift+Enter.

Alternative options that already work:
- Ctrl+J for newlines
- Alt+Enter for newlines (macOS Terminal)

Supported terminals with known configuration:
- VSCode, Cursor, Windsurf
- Ghostty, WezTerm, Kitty, Alacritty
- iTerm2

For other terminals, consult your terminal's documentation on custom key bindings.`
	}
}
