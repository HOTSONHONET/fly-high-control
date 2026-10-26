package announce

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Runner struct {
	AudioDevice string
}

func NewRunner(audioDevice string) *Runner {
	return &Runner{AudioDevice: strings.TrimSpace(audioDevice)}
}

func (r *Runner) RunLocal(ctx context.Context, message string, repeat int) error {
	for i := 0; i < repeat; i++ {
		if err := r.runOnce(ctx, message); err != nil {
			return fmt.Errorf("repeat %d/%d: %w", i+1, repeat, err)
		}
	}
	return nil
}

func (r *Runner) runOnce(ctx context.Context, message string) error {
	tmpDir, err := os.MkdirTemp("", "fly-high-control-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	wav := filepath.Join(tmpDir, "announcement.wav")
	if output, err := exec.CommandContext(ctx, "espeak-ng", "-w", wav, message).CombinedOutput(); err != nil {
		return fmt.Errorf("espeak-ng: %w: %s", err, strings.TrimSpace(string(output)))
	}

	args := []string{}
	if r.AudioDevice != "" {
		args = append(args, "-D", r.AudioDevice)
	}
	args = append(args, wav)

	if output, err := exec.CommandContext(ctx, "aplay", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("aplay: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
