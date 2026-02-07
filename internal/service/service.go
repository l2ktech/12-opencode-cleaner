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
	IntervalSec      int
	MinAgeMin        int
	WatchIntervalSec int
	WarnMemMB        int
	BinPath          string
}

func Install(cfg Config) error {
	if cfg.IntervalSec <= 0 || cfg.MinAgeMin <= 0 || cfg.WatchIntervalSec <= 0 || cfg.WarnMemMB <= 0 || cfg.BinPath == "" {
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
		fmt.Println("[clean 服务]")
		if err := runAndPipe("launchctl", "print", fmt.Sprintf("gui/%d/com.wzy.oc-cleaner.clean", os.Getuid())); err != nil {
			return err
		}
		fmt.Println("\n[watch 服务]")
		return runAndPipe("launchctl", "print", fmt.Sprintf("gui/%d/com.wzy.oc-cleaner.watch", os.Getuid()))
	case "linux":
		fmt.Println("[clean timer]")
		if err := runAndPipe("systemctl", "--user", "status", "oc-cleaner-clean.timer", "--no-pager"); err != nil {
			return err
		}
		fmt.Println("\n[watch timer]")
		return runAndPipe("systemctl", "--user", "status", "oc-cleaner-watch.timer", "--no-pager")
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
	cleanPlist := filepath.Join(agentDir, "com.wzy.oc-cleaner.clean.plist")
	watchPlist := filepath.Join(agentDir, "com.wzy.oc-cleaner.watch.plist")

	cleanContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.wzy.oc-cleaner.clean</string>
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
		filepath.Join(home, "Library", "Logs", "oc-cleaner-clean.log"),
		filepath.Join(home, "Library", "Logs", "oc-cleaner-clean.err.log"),
	)

	watchContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.wzy.oc-cleaner.watch</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>watch</string>
    <string>--warn-mem-mb</string>
    <string>%d</string>
    <string>--once</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>StartInterval</key><integer>%d</integer>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, cfg.BinPath, cfg.WarnMemMB, cfg.WatchIntervalSec,
		filepath.Join(home, "Library", "Logs", "oc-cleaner-watch.log"),
		filepath.Join(home, "Library", "Logs", "oc-cleaner-watch.err.log"),
	)

	if err := os.WriteFile(cleanPlist, []byte(cleanContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(watchPlist, []byte(watchContent), 0o644); err != nil {
		return err
	}

	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), cleanPlist).Run()
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), watchPlist).Run()

	if err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), cleanPlist).Run(); err != nil {
		return fmt.Errorf("clean bootstrap 失败: %w", err)
	}
	if err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), watchPlist).Run(); err != nil {
		return fmt.Errorf("watch bootstrap 失败: %w", err)
	}
	if err := exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/com.wzy.oc-cleaner.clean", os.Getuid())).Run(); err != nil {
		return fmt.Errorf("clean kickstart 失败: %w", err)
	}
	if err := exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/com.wzy.oc-cleaner.watch", os.Getuid())).Run(); err != nil {
		return fmt.Errorf("watch kickstart 失败: %w", err)
	}
	return nil
}

func uninstallDarwin() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cleanPlist := filepath.Join(home, "Library", "LaunchAgents", "com.wzy.oc-cleaner.clean.plist")
	watchPlist := filepath.Join(home, "Library", "LaunchAgents", "com.wzy.oc-cleaner.watch.plist")
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), cleanPlist).Run()
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), watchPlist).Run()
	if err := os.Remove(cleanPlist); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(watchPlist); err != nil && !os.IsNotExist(err) {
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

	cleanSvcPath := filepath.Join(dir, "oc-cleaner-clean.service")
	cleanTimerPath := filepath.Join(dir, "oc-cleaner-clean.timer")
	watchSvcPath := filepath.Join(dir, "oc-cleaner-watch.service")
	watchTimerPath := filepath.Join(dir, "oc-cleaner-watch.timer")

	cleanSvc := fmt.Sprintf(`[Unit]
Description=OC Cleaner

[Service]
Type=oneshot
ExecStart=%s clean --min-age-min %d
`, cfg.BinPath, cfg.MinAgeMin)

	cleanTimer := fmt.Sprintf(`[Unit]
Description=Run OC Cleaner periodically

[Timer]
OnBootSec=30s
OnUnitActiveSec=%ss
Unit=oc-cleaner-clean.service

[Install]
WantedBy=timers.target
`, strconv.Itoa(cfg.IntervalSec))

	watchSvc := fmt.Sprintf(`[Unit]
Description=OC Cleaner Watch

[Service]
Type=oneshot
ExecStart=%s watch --warn-mem-mb %d --once
`, cfg.BinPath, cfg.WarnMemMB)

	watchTimer := fmt.Sprintf(`[Unit]
Description=Run OC Cleaner watch periodically

[Timer]
OnBootSec=30s
OnUnitActiveSec=%ss
Unit=oc-cleaner-watch.service

[Install]
WantedBy=timers.target
`, strconv.Itoa(cfg.WatchIntervalSec))

	if err := os.WriteFile(cleanSvcPath, []byte(cleanSvc), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(cleanTimerPath, []byte(cleanTimer), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(watchSvcPath, []byte(watchSvc), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(watchTimerPath, []byte(watchTimer), 0o644); err != nil {
		return err
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return err
	}
	if err := exec.Command("systemctl", "--user", "enable", "--now", "oc-cleaner-clean.timer").Run(); err != nil {
		return err
	}
	if err := exec.Command("systemctl", "--user", "enable", "--now", "oc-cleaner-watch.timer").Run(); err != nil {
		return err
	}
	return nil
}

func uninstallLinux() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", "oc-cleaner-clean.timer").Run()
	_ = exec.Command("systemctl", "--user", "disable", "--now", "oc-cleaner-watch.timer").Run()
	dir := filepath.Join(home, ".config", "systemd", "user")
	_ = os.Remove(filepath.Join(dir, "oc-cleaner-clean.timer"))
	_ = os.Remove(filepath.Join(dir, "oc-cleaner-clean.service"))
	_ = os.Remove(filepath.Join(dir, "oc-cleaner-watch.timer"))
	_ = os.Remove(filepath.Join(dir, "oc-cleaner-watch.service"))
	return exec.Command("systemctl", "--user", "daemon-reload").Run()
}

func runAndPipe(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
