package announce

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Target struct {
	Name        string
	Mode        string
	SSHHost     string
	AudioDevice string
}

type Runner struct {
	targets map[string]Target
}

func NewRunner(targets []Target) *Runner {
	m := make(map[string]Target, len(targets))
	for _, target := range targets {
		m[strings.ToLower(target.Name)] = target
	}
	return &Runner{targets: m}
}

func (r *Runner) Run(ctx context.Context, cmd Command) error {
	if cmd.Target == "all" {
		var errs []string
		for _, target := range r.targets {
			if err := r.runRepeated(ctx, target, cmd.Message, cmd.Repeat); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", target.Name, err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return nil
	}

	target, ok := r.targets[cmd.Target]
	if !ok {
		return fmt.Errorf("target %q is not configured", cmd.Target)
	}
	return r.runRepeated(ctx, target, cmd.Message, cmd.Repeat)
}

func (r *Runner) runRepeated(ctx context.Context, target Target, message string, repeat int) error {
	for i := 0; i < repeat; i++ {
		if err := r.runTarget(ctx, target, message); err != nil {
			return fmt.Errorf("repeat %d/%d: %w", i+1, repeat, err)
		}
	}
	return nil
}

func (r *Runner) runTarget(ctx context.Context, target Target, message string) error {
	switch strings.ToLower(target.Mode) {
	case "local":
		return runLocal(ctx, target, message)
	case "ssh":
		return runSSH(ctx, target, message)
	default:
		return fmt.Errorf("target %q has unsupported mode %q", target.Name, target.Mode)
	}
}

func runLocal(ctx context.Context, target Target, message string) error {
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
	if strings.TrimSpace(target.AudioDevice) != "" {
		args = append(args, "-D", target.AudioDevice)
	}
	args = append(args, wav)

	if output, err := exec.CommandContext(ctx, "aplay", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("aplay: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runSSH(ctx context.Context, target Target, message string) error {
	if strings.TrimSpace(target.SSHHost) == "" {
		return fmt.Errorf("SSH host is empty")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(message))
	device := strings.TrimSpace(target.AudioDevice)
	if device == "" {
		device = "default"
	}

	remote := fmt.Sprintf(
		"tmp=$(mktemp /tmp/fly-high-control-XXXXXX.wav) && printf %%s %s | base64 -d | espeak-ng -w \"$tmp\" && aplay -D %s \"$tmp\"; rc=$?; rm -f \"$tmp\"; exit $rc",
		shellQuote(encoded),
		shellQuote(device),
	)

	if output, err := exec.CommandContext(ctx, "ssh", target.SSHHost, remote).CombinedOutput(); err != nil {
		return fmt.Errorf("ssh: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
