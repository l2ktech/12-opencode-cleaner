package proc

import "strings"

// ParseETimeToMinutes 支持: mm:ss, hh:mm:ss, dd-hh:mm:ss
func ParseETimeToMinutes(etime string) int {
	etime = strings.TrimSpace(etime)
	if etime == "" {
		return 0
	}

	if strings.Contains(etime, "-") {
		p := strings.SplitN(etime, "-", 2)
		d := atoiSafe(p[0])
		h, m, s := parseHMS(p[1])
		total := d*24*60 + h*60 + m
		if s >= 30 {
			total++
		}
		return total
	}

	parts := strings.Split(etime, ":")
	switch len(parts) {
	case 2:
		m := atoiSafe(parts[0])
		s := atoiSafe(parts[1])
		if s >= 30 {
			m++
		}
		return m
	case 3:
		h := atoiSafe(parts[0])
		m := atoiSafe(parts[1])
		s := atoiSafe(parts[2])
		total := h*60 + m
		if s >= 30 {
			total++
		}
		return total
	default:
		return 0
	}
}

func parseHMS(v string) (int, int, int) {
	parts := strings.Split(v, ":")
	if len(parts) != 3 {
		return 0, 0, 0
	}
	return atoiSafe(parts[0]), atoiSafe(parts[1]), atoiSafe(parts[2])
}

func atoiSafe(s string) int {
	n := 0
	for _, ch := range strings.TrimSpace(s) {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
