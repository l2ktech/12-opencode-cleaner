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
	fmt.Printf("%-8s %-8s %-8s %-10s %-10s %s\n", "PID", "PPID", "TTY", "AGE_MIN", "NET", "COMMAND")
	for _, p := range all {
		net := "-"
		if p.NetCheck {
			if p.HasNet {
				net = "YES"
			} else {
				net = "NO"
			}
		}
		fmt.Printf("%-8d %-8d %-8s %-10d %-10s %s\n", p.PID, p.PPID, p.TTY, p.AgeMin, net, p.Command)
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
