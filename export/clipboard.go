package export

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
)

type ClipboardBackend interface {
	Copy(text string) error
	Available() bool
}

type ClipboardManager struct {
	backends []ClipboardBackend
}

func NewClipboardManager() *ClipboardManager {
	return &ClipboardManager{
		backends: []ClipboardBackend{
			&OSC52Backend{},
			&WaylandBackend{},
			&X11Backend{},
			&FileBackend{},
		},
	}
}

func (cm *ClipboardManager) Copy(text string) error {
	for _, backend := range cm.backends {
		if backend.Available() {
			if err := backend.Copy(text); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("no clipboard backend available")
}

// OSC 52 - Works over SSH
type OSC52Backend struct{}

func (b *OSC52Backend) Available() bool {
	return true
}

func (b *OSC52Backend) Copy(text string) error {
	// Limit to 100KB for terminal compatibility
	if len(text) > 100*1024 {
		return fmt.Errorf("content too large for OSC52")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := fmt.Sprintf("\033]52;c;%s\033\\", encoded)
	_, err := os.Stdout.Write([]byte(seq))
	return err
}

// Wayland
type WaylandBackend struct{}

func (b *WaylandBackend) Available() bool {
	_, err := exec.LookPath("wl-copy")
	return err == nil && os.Getenv("WAYLAND_DISPLAY") != ""
}

func (b *WaylandBackend) Copy(text string) error {
	cmd := exec.Command("wl-copy")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write([]byte(text)); err != nil {
		stdin.Close()
		return err
	}

	stdin.Close()
	return cmd.Wait()
}

// X11
type X11Backend struct{}

func (b *X11Backend) Available() bool {
	_, err := exec.LookPath("xclip")
	return err == nil && os.Getenv("DISPLAY") != ""
}

func (b *X11Backend) Copy(text string) error {
	cmd := exec.Command("xclip", "-selection", "clipboard")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write([]byte(text)); err != nil {
		return err
	}

	stdin.Close()
	return cmd.Wait()
}

// File fallback
type FileBackend struct{}

func (b *FileBackend) Available() bool {
	return true
}

func (b *FileBackend) Copy(text string) error {
	path := "/tmp/rush-clipboard.md"
	return os.WriteFile(path, []byte(text), 0644)
}
