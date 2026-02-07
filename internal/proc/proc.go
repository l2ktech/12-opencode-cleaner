package proc

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Process struct {
	PID      int
	PPID     int
	TTY      string
	ETime    string
	Command  string
	AgeMin   int
	HasNet   bool
	NetCheck bool
}

type Candidate struct {
	Process Process
	Reason  string
}

type ScanOptions struct {
	MinAgeMin             int
	ProtectNonLoopbackNet bool
}

func Scan(opts ScanOptions) ([]Process, []Candidate, error) {
	all, err := listOpencodeProcesses()
	if err != nil {
		return nil, nil, err
	}

	candidates := make([]Candidate, 0)
	for i := range all {
		p := &all[i]
		p.AgeMin = ParseETimeToMinutes(p.ETime)

		if opts.ProtectNonLoopbackNet {
			has, checked := hasNonLoopbackEstablished(p.PID)
			p.HasNet = has
			p.NetCheck = checked
		}

		if ok, reason := shouldCandidate(*p, opts); ok {
			candidates = append(candidates, Candidate{Process: *p, Reason: reason})
		}
	}

	return all, candidates, nil
}

func listOpencodeProcesses() ([]Process, error) {
	cmd := exec.Command("ps", "-axo", "pid=,ppid=,tty=,etime=,command=")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 ps 失败: %w", err)
	}

	ret := make([]Process, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !isTargetCommand(line) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}

		etime := fields[3]
		cmdline := strings.TrimSpace(strings.SplitN(line, etime, 2)[1])

		ret = append(ret, Process{
			PID:     pid,
			PPID:    ppid,
			TTY:     fields[2],
			ETime:   etime,
			Command: cmdline,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 ps 输出失败: %w", err)
	}
	return ret, nil
}

func isTargetCommand(line string) bool {
	return strings.Contains(line, "opencode")
}

func isNoTTY(tty string) bool {
	return tty == "??" || tty == "?"
}

func hasNonLoopbackEstablished(pid int) (bool, bool) {
	cmd := exec.Command("lsof", "-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:ESTABLISHED")
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			continue
		}
		if strings.Contains(line, "127.0.0.1") || strings.Contains(line, "[::1]") {
			continue
		}
		if strings.Contains(line, "->") {
			return true, true
		}
	}
	return false, true
}

func shouldCandidate(p Process, opts ScanOptions) (bool, string) {
	if p.PPID != 1 {
		return false, "ppid!=1"
	}
	if !isNoTTY(p.TTY) {
		return false, "tty_attached"
	}
	if p.AgeMin < opts.MinAgeMin {
		return false, "age_too_low"
	}
	if opts.ProtectNonLoopbackNet {
		if !p.NetCheck {
			return false, "net_check_unavailable"
		}
		if p.HasNet {
			return false, "has_non_loopback_net"
		}
	}
	return true, "ppid=1+no_tty+age_ok+safe_net"
}
