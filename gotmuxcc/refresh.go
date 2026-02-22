package gotmuxcc

import "fmt"

// SetClientSize sets the overall terminal size for this control client.
// This is equivalent to `refresh-client -C WxH`.
func (t *Tmux) SetClientSize(width, height int) error {
	cmd := fmt.Sprintf("refresh-client -C %dx%d", width, height)
	_, err := t.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set client size: %w", err)
	}
	return nil
}

// SetWindowSize sets the size for a specific window on this control client.
// The windowID should include the @ prefix (e.g. "@0").
// This is equivalent to `refresh-client -C @wid:WxH`.
func (t *Tmux) SetWindowSize(windowID string, width, height int) error {
	cmd := fmt.Sprintf("refresh-client -C %s:%dx%d", windowID, width, height)
	_, err := t.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set window size: %w", err)
	}
	return nil
}

// ClearWindowSize clears a per-window size override, reverting to the client size.
// The windowID should include the @ prefix (e.g. "@0").
// This is equivalent to `refresh-client -C @wid:`.
func (t *Tmux) ClearWindowSize(windowID string) error {
	cmd := fmt.Sprintf("refresh-client -C %s:", windowID)
	_, err := t.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to clear window size: %w", err)
	}
	return nil
}
