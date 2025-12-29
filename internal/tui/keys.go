package tui

import (
	"github.com/charmbracelet/bubbletea"
)

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	// Navigation
	Up       Key
	Down     Key
	Left     Key
	Right    Key
	PageUp   Key
	PageDown Key
	Home     Key
	End      Key

	// Actions
	Select Key
	Back   Key
	Quit   Key
	Help   Key
	Search Key

	// Function keys for module navigation
	F1  Key
	F2  Key
	F3  Key
	F4  Key
	F5  Key
	F6  Key
	F7  Key
	F8  Key
	F9  Key
	F10 Key

	// Form navigation
	Tab      Key
	ShiftTab Key
	Enter    Key
	Escape   Key

	// Editing
	Delete    Key
	Backspace Key
}

// Key represents a key binding.
type Key struct {
	Keys    []string
	Help    string
	Enabled bool
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: Key{
			Keys:    []string{"up", "k"},
			Help:    "up",
			Enabled: true,
		},
		Down: Key{
			Keys:    []string{"down", "j"},
			Help:    "down",
			Enabled: true,
		},
		Left: Key{
			Keys:    []string{"left", "h"},
			Help:    "left",
			Enabled: true,
		},
		Right: Key{
			Keys:    []string{"right", "l"},
			Help:    "right",
			Enabled: true,
		},
		PageUp: Key{
			Keys:    []string{"pgup", "ctrl+u"},
			Help:    "page up",
			Enabled: true,
		},
		PageDown: Key{
			Keys:    []string{"pgdown", "ctrl+d"},
			Help:    "page down",
			Enabled: true,
		},
		Home: Key{
			Keys:    []string{"home", "g"},
			Help:    "home",
			Enabled: true,
		},
		End: Key{
			Keys:    []string{"end", "G"},
			Help:    "end",
			Enabled: true,
		},

		// Actions
		Select: Key{
			Keys:    []string{"enter", " "},
			Help:    "select",
			Enabled: true,
		},
		Back: Key{
			Keys:    []string{"esc", "backspace"},
			Help:    "back",
			Enabled: true,
		},
		Quit: Key{
			Keys:    []string{"q", "ctrl+c"},
			Help:    "quit",
			Enabled: true,
		},
		Help: Key{
			Keys:    []string{"?", "f1"},
			Help:    "help",
			Enabled: true,
		},
		Search: Key{
			Keys:    []string{"/"},
			Help:    "search",
			Enabled: true,
		},

		// Function keys
		F1: Key{
			Keys:    []string{"f1"},
			Help:    "Help",
			Enabled: true,
		},
		F2: Key{
			Keys:    []string{"f2"},
			Help:    "Dashboard",
			Enabled: true,
		},
		F3: Key{
			Keys:    []string{"f3"},
			Help:    "Population",
			Enabled: true,
		},
		F4: Key{
			Keys:    []string{"f4"},
			Help:    "Resources",
			Enabled: true,
		},
		F5: Key{
			Keys:    []string{"f5"},
			Help:    "Facilities",
			Enabled: true,
		},
		F6: Key{
			Keys:    []string{"f6"},
			Help:    "Labor",
			Enabled: true,
		},
		F7: Key{
			Keys:    []string{"f7"},
			Help:    "Medical",
			Enabled: true,
		},
		F8: Key{
			Keys:    []string{"f8"},
			Help:    "Security",
			Enabled: true,
		},
		F9: Key{
			Keys:    []string{"f9"},
			Help:    "Governance",
			Enabled: true,
		},
		F10: Key{
			Keys:    []string{"f10"},
			Help:    "Quit",
			Enabled: true,
		},

		// Form navigation
		Tab: Key{
			Keys:    []string{"tab"},
			Help:    "next field",
			Enabled: true,
		},
		ShiftTab: Key{
			Keys:    []string{"shift+tab"},
			Help:    "prev field",
			Enabled: true,
		},
		Enter: Key{
			Keys:    []string{"enter"},
			Help:    "confirm",
			Enabled: true,
		},
		Escape: Key{
			Keys:    []string{"esc"},
			Help:    "cancel",
			Enabled: true,
		},

		// Editing
		Delete: Key{
			Keys:    []string{"delete"},
			Help:    "delete",
			Enabled: true,
		},
		Backspace: Key{
			Keys:    []string{"backspace"},
			Help:    "backspace",
			Enabled: true,
		},
	}
}

// Matches checks if a key message matches this key binding.
func (k Key) Matches(msg tea.KeyMsg) bool {
	if !k.Enabled {
		return false
	}

	keyStr := msg.String()
	for _, key := range k.Keys {
		if keyStr == key {
			return true
		}
	}
	return false
}

// MatchesAny checks if a key message matches any of the provided key bindings.
func MatchesAny(msg tea.KeyMsg, keys ...Key) bool {
	for _, k := range keys {
		if k.Matches(msg) {
			return true
		}
	}
	return false
}

// IsQuit checks if the key message is a quit command.
func (km KeyMap) IsQuit(msg tea.KeyMsg) bool {
	return km.Quit.Matches(msg) || km.F10.Matches(msg)
}

// IsNavigation checks if the key message is a navigation key.
func (km KeyMap) IsNavigation(msg tea.KeyMsg) bool {
	return MatchesAny(msg, km.Up, km.Down, km.Left, km.Right,
		km.PageUp, km.PageDown, km.Home, km.End)
}

// IsFunctionKey checks if the key message is a function key.
func (km KeyMap) IsFunctionKey(msg tea.KeyMsg) bool {
	return MatchesAny(msg, km.F1, km.F2, km.F3, km.F4, km.F5,
		km.F6, km.F7, km.F8, km.F9, km.F10)
}

// GetFunctionKeyModule returns the module name for a function key.
func (km KeyMap) GetFunctionKeyModule(msg tea.KeyMsg) string {
	switch {
	case km.F1.Matches(msg):
		return "help"
	case km.F2.Matches(msg):
		return "dashboard"
	case km.F3.Matches(msg):
		return "population"
	case km.F4.Matches(msg):
		return "resources"
	case km.F5.Matches(msg):
		return "facilities"
	case km.F6.Matches(msg):
		return "labor"
	case km.F7.Matches(msg):
		return "medical"
	case km.F8.Matches(msg):
		return "security"
	case km.F9.Matches(msg):
		return "governance"
	case km.F10.Matches(msg):
		return "quit"
	default:
		return ""
	}
}

// StatusBarHelp returns the help text for the status bar.
func (km KeyMap) StatusBarHelp() string {
	return "[F1]Help [F2]Dashboard [F3]Population [F4]Resources [F5]Facilities [F10]Quit"
}
