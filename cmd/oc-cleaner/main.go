package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/l2ktech/opencode-cleaner/internal/proc"
	"github.com/l2ktech/opencode-cleaner/internal/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:])
	case "clean":
		return runClean(args[1:])
	case "watch":
		return runWatch(args[1:])
	case "service":
		return runService(args[1:])
	default:
		return usage()
	}
}

func usage() error {
	fmt.Println(`oc-cleaner 用法:
  oc-cleaner scan [--min-age-min 10] [--json]
  oc-cleaner clean [--min-age-min 10] [--kill-after-sec 0]
  oc-cleaner watch [--warn-mem-mb 1024] [--interval-sec 30] [--once]
  oc-cleaner service install [--interval-sec 120] [--min-age-min 10]
  oc-cleaner service uninstall
  oc-cleaner service status`)
	return errors.New("参数不完整")
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	minAge := fs.Int("min-age-min", 10, "最小存活分钟")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	if err := fs.Parse(args); err != nil {
		return err
	}

	all, cands, err := proc.Scan(proc.ScanOptions{MinAgeMin: *minAge, ProtectNonLoopbackNet: true})
	if err != nil {
		return err
	}

	if *jsonOut {
		payload := map[string]any{"all": all, "candidates": cands}
		b, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("总进程: %d, 可清理候选: %d\n", len(all), len(cands))
	fmt.Printf("%-8s %-8s %-8s %-10s %-10s %-10s %s\n", "PID", "PPID", "TTY", "AGE_MIN", "NET", "RSS(MB)", "COMMAND")
	for _, p := range all {
		net := "-"
		if p.NetCheck {
			if p.HasNet {
				net = "YES"
			} else {
				net = "NO"
			}
		}
		rssMB := float64(p.RSSKB) / 1024.0
		fmt.Printf("%-8d %-8d %-8s %-10d %-10s %-10.1f %s\n", p.PID, p.PPID, p.TTY, p.AgeMin, net, rssMB, p.Command)
	}
	return nil
}

func runClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	minAge := fs.Int("min-age-min", 10, "最小存活分钟")
	killAfter := fs.Int("kill-after-sec", 0, "TERM 后升级 KILL 秒数，0=不升级")
	if err := fs.Parse(args); err != nil {
		return err
	}

	_, cands, err := proc.Scan(proc.ScanOptions{MinAgeMin: *minAge, ProtectNonLoopbackNet: true})
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		fmt.Println("没有可清理候选")
		return nil
	}

	fmt.Printf("将温和关闭 %d 个候选进程\n", len(cands))
	for _, c := range cands {
		_ = syscall.Kill(c.Process.PID, syscall.SIGTERM)
		fmt.Printf("TERM pid=%d cmd=%s\n", c.Process.PID, c.Process.Command)
	}

	if *killAfter > 0 {
		time.Sleep(time.Duration(*killAfter) * time.Second)
		for _, c := range cands {
			if err := syscall.Kill(c.Process.PID, 0); err == nil {
				_ = syscall.Kill(c.Process.PID, syscall.SIGKILL)
				fmt.Printf("KILL pid=%d\n", c.Process.PID)
			}
		}
	}

	return nil
}

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	warnMemMB := fs.Int("warn-mem-mb", 1024, "内存告警阈值(MB，仅提醒不重启)")
	intervalSec := fs.Int("interval-sec", 30, "轮询间隔秒")
	once := fs.Bool("once", false, "仅检测一次")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *warnMemMB <= 0 {
		return errors.New("warn-mem-mb 必须大于 0")
	}
	if *intervalSec <= 0 {
		return errors.New("interval-sec 必须大于 0")
	}

	for {
		all, _, err := proc.Scan(proc.ScanOptions{MinAgeMin: 0, ProtectNonLoopbackNet: false})
		if err != nil {
			return err
		}
		high := proc.FilterHighMemory(all, *warnMemMB)

		if *jsonOut {
			payload := map[string]any{
				"time":          time.Now().Format(time.RFC3339),
				"warn_mem_mb":   *warnMemMB,
				"process_count": len(all),
				"warn_count":    len(high),
				"warnings":      high,
			}
			b, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Println(string(b))
		} else {
			printWatchResult(high, *warnMemMB)
		}

		if *once {
			return nil
		}
		time.Sleep(time.Duration(*intervalSec) * time.Second)
	}
}

func printWatchResult(high []proc.Process, warnMemMB int) {
	now := time.Now().Format("2006-01-02 15:04:05")
	if len(high) == 0 {
		fmt.Printf("[%s] OK: 没有进程超过 %dMB\n", now, warnMemMB)
		return
	}

	fmt.Printf("[%s] WARN: 发现 %d 个进程超过 %dMB（仅提醒，不重启）\n", now, len(high), warnMemMB)
	fmt.Printf("%-8s %-8s %-8s %-10s %s\n", "PID", "PPID", "TTY", "RSS(MB)", "COMMAND")
	for _, p := range high {
		rssMB := float64(p.RSSKB) / 1024.0
		fmt.Printf("%-8d %-8d %-8s %-10.1f %s\n", p.PID, p.PPID, p.TTY, rssMB, p.Command)
	}
}

func runService(args []string) error {
	if len(args) == 0 {
		return errors.New("service 缺少子命令")
	}
	switch args[0] {
	case "install":
		fs := flag.NewFlagSet("service install", flag.ContinueOnError)
		interval := fs.Int("interval-sec", 120, "扫描间隔秒")
		minAge := fs.Int("min-age-min", 10, "最小存活分钟")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if err := service.Install(service.Config{IntervalSec: *interval, MinAgeMin: *minAge, BinPath: exe}); err != nil {
			return err
		}
		fmt.Println("服务安装完成")
		return nil
	case "uninstall":
		if err := service.Uninstall(); err != nil {
			return err
		}
		fmt.Println("服务卸载完成")
		return nil
	case "status":
		return service.Status()
	default:
		return errors.New("未知 service 子命令")
	}
}

func init() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		os.Exit(0)
	}()
}
