package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

type Config struct {
	IntervalSec int
	MinAgeMin   int
	BinPath     string
}

func Install(cfg Config) error {
	if cfg.IntervalSec <= 0 || cfg.MinAgeMin <= 0 || cfg.BinPath == "" {
		return errors.New("参数无效")
	}
	switch runtime.GOOS {
	case "darwin":
		return installDarwin(cfg)
	case "linux":
		return installLinux(cfg)
	default:
		return fmt.Errorf("暂不支持系统: %s", runtime.GOOS)
	}
}

func Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallDarwin()
	case "linux":
		return uninstallLinux()
	default:
		return fmt.Errorf("暂不支持系统: %s", runtime.GOOS)
	}
}

func Status() error {
	switch runtime.GOOS {
	case "darwin":
		return runAndPipe("launchctl", "print", fmt.Sprintf("gui/%d/com.wzy.oc-cleaner", os.Getuid()))
	case "linux":
		return runAndPipe("systemctl", "--user", "status", "oc-cleaner.timer", "--no-pager")
	default:
		return fmt.Errorf("暂不支持系统: %s", runtime.GOOS)
	}
}

func installDarwin(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	agentDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return err
	}
	plist := filepath.Join(agentDir, "com.wzy.oc-cleaner.plist")

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.wzy.oc-cleaner</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>clean</string>
    <string>--min-age-min</string>
    <string>%d</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>StartInterval</key><integer>%d</integer>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, cfg.BinPath, cfg.MinAgeMin, cfg.IntervalSec,
		filepath.Join(home, "Library", "Logs", "oc-cleaner.log"),
		filepath.Join(home, "Library", "Logs", "oc-cleaner.err.log"),
	)
	if err := os.WriteFile(plist, []byte(content), 0o644); err != nil {
		return err
	}

	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), plist).Run()
	if err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), plist).Run(); err != nil {
		return fmt.Errorf("bootstrap 失败: %w", err)
	}
	if err := exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/com.wzy.oc-cleaner", os.Getuid())).Run(); err != nil {
		return fmt.Errorf("kickstart 失败: %w", err)
	}
	return nil
}

func uninstallDarwin() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", "com.wzy.oc-cleaner.plist")
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), plist).Run()
	if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func installLinux(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	svcPath := filepath.Join(dir, "oc-cleaner.service")
	timerPath := filepath.Join(dir, "oc-cleaner.timer")
	svc := fmt.Sprintf(`[Unit]
Description=OC Cleaner

[Service]
Type=oneshot
ExecStart=%s clean --min-age-min %d
`, cfg.BinPath, cfg.MinAgeMin)

	timer := fmt.Sprintf(`[Unit]
Description=Run OC Cleaner periodically

[Timer]
OnBootSec=30s
OnUnitActiveSec=%ss
Unit=oc-cleaner.service

[Install]
WantedBy=timers.target
`, strconv.Itoa(cfg.IntervalSec))

	if err := os.WriteFile(svcPath, []byte(svc), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(timerPath, []byte(timer), 0o644); err != nil {
		return err
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return err
	}
	if err := exec.Command("systemctl", "--user", "enable", "--now", "oc-cleaner.timer").Run(); err != nil {
		return err
	}
	return nil
}

func uninstallLinux() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", "oc-cleaner.timer").Run()
	dir := filepath.Join(home, ".config", "systemd", "user")
	_ = os.Remove(filepath.Join(dir, "oc-cleaner.timer"))
	_ = os.Remove(filepath.Join(dir, "oc-cleaner.service"))
	return exec.Command("systemctl", "--user", "daemon-reload").Run()
}

func runAndPipe(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
