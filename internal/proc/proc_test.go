package proc

import "testing"

func TestShouldCandidate(t *testing.T) {
	opts := ScanOptions{MinAgeMin: 10, ProtectNonLoopbackNet: true}

	p := Process{PID: 1, PPID: 1, TTY: "??", AgeMin: 20, NetCheck: true, HasNet: false}
	ok, _ := shouldCandidate(p, opts)
	if !ok {
		t.Fatalf("应为候选")
	}

	p = Process{PID: 2, PPID: 100, TTY: "??", AgeMin: 20, NetCheck: true, HasNet: false}
	ok, _ = shouldCandidate(p, opts)
	if ok {
		t.Fatalf("ppid 非 1 不应为候选")
	}

	p = Process{PID: 3, PPID: 1, TTY: "ttys001", AgeMin: 20, NetCheck: true, HasNet: false}
	ok, _ = shouldCandidate(p, opts)
	if ok {
		t.Fatalf("有 tty 不应为候选")
	}

	p = Process{PID: 4, PPID: 1, TTY: "??", AgeMin: 5, NetCheck: true, HasNet: false}
	ok, _ = shouldCandidate(p, opts)
	if ok {
		t.Fatalf("年龄不足不应为候选")
	}

	p = Process{PID: 5, PPID: 1, TTY: "??", AgeMin: 20, NetCheck: true, HasNet: true}
	ok, _ = shouldCandidate(p, opts)
	if ok {
		t.Fatalf("存在非本地连接不应为候选")
	}
}
