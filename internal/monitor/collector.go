package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Report struct {
	Node    string `json:"node"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

type Collector struct {
	node string
}

func New(node string) *Collector {
	return &Collector{node: node}
}

func (c *Collector) Collect(ctx context.Context, kind string) Report {
	kind = strings.ToLower(strings.TrimSpace(kind))
	var summary string

	switch kind {
	case "status":
		summary = c.status(ctx)
	case "gpu":
		summary = c.gpu(ctx)
	case "disk":
		summary = c.disk()
	case "docker":
		summary = c.docker(ctx)
	case "uptime":
		summary = c.uptime()
	case "battery":
		summary = c.battery()
	default:
		summary = fmt.Sprintf("unsupported monitor kind %q", kind)
	}

	return Report{Node: c.node, Kind: kind, Summary: summary}
}

func (c *Collector) status(ctx context.Context) string {
	parts := []string{
		fmt.Sprintf("Uptime: %s", c.uptime()),
		fmt.Sprintf("Load: %s", loadAverage()),
		fmt.Sprintf("CPU cores: %d", runtime.NumCPU()),
		fmt.Sprintf("Memory: %s", memorySummary()),
		fmt.Sprintf("Disk /: %s", diskSummary("/")),
		fmt.Sprintf("Battery: %s", c.battery()),
	}

	gpu := c.gpu(ctx)
	if !strings.HasPrefix(gpu, "unavailable") {
		parts = append(parts, "GPU: "+gpu)
	}
	return strings.Join(parts, "\n")
}

func (c *Collector) gpu(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,temperature.gpu,utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits",
	)
	out, err := cmd.Output()
	if err != nil {
		return "unavailable on this node/container"
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return "unavailable on this node/container"
	}
	fields := strings.Split(line, ",")
	if len(fields) < 5 {
		return line
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return fmt.Sprintf("%s | %s°C | util %s%% | VRAM %s/%s MiB",
		fields[0], fields[1], fields[2], fields[3], fields[4])
}

func (c *Collector) disk() string {
	return diskSummary("/")
}

func (c *Collector) uptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unavailable"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "unavailable"
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "unavailable"
	}

	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func (c *Collector) battery() string {
	matches, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(matches) == 0 {
		return "not present"
	}

	var values []string
	for _, bat := range matches {
		name := filepath.Base(bat)
		capacity := readTrim(filepath.Join(bat, "capacity"))
		status := readTrim(filepath.Join(bat, "status"))
		if capacity == "" {
			capacity = "?"
		}
		if status == "" {
			status = "unknown"
		}
		values = append(values, fmt.Sprintf("%s %s%% (%s)", name, capacity, status))
	}
	return strings.Join(values, ", ")
}

func (c *Collector) docker(ctx context.Context) string {
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return "Docker socket unavailable"
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", "/var/run/docker.sock")
		},
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?all=1", nil)
	if err != nil {
		return "Docker query failed"
	}
	resp, err := client.Do(req)
	if err != nil {
		return "Docker query failed: " + err.Error()
	}
	defer resp.Body.Close()

	var containers []struct {
		Names  []string `json:"Names"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return "Docker response decode failed"
	}
	if len(containers) == 0 {
		return "No containers"
	}

	lines := make([]string, 0, len(containers))
	for _, container := range containers {
		name := "unknown"
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		lines = append(lines, fmt.Sprintf("%s — %s — %s", name, container.State, container.Status))
	}
	return strings.Join(lines, "\n")
}

func loadAverage() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "unavailable"
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return "unavailable"
	}
	return strings.Join(fields[:3], " ")
}

func memorySummary() string {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return "unavailable"
	}
	defer file.Close()

	var totalKB, availableKB uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalKB = value
		case "MemAvailable":
			availableKB = value
		}
	}
	if totalKB == 0 {
		return "unavailable"
	}
	usedKB := totalKB - availableKB
	return fmt.Sprintf("%.1f/%.1f GiB (%.0f%%)",
		float64(usedKB)/1024/1024,
		float64(totalKB)/1024/1024,
		float64(usedKB)*100/float64(totalKB),
	)
}

func diskSummary(path string) string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return "unavailable"
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	if total == 0 {
		return "unavailable"
	}
	return fmt.Sprintf("%.1f/%.1f GiB (%.0f%%)",
		float64(used)/(1<<30),
		float64(total)/(1<<30),
		float64(used)*100/float64(total),
	)
}

func readTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
