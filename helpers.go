package main

import "fmt"

func truncate(s string, maxLen int) string {
	if maxLen > 3 && len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func marquee(s string, maxLen, step int) string {
	if len(s) <= maxLen {
		return s
	}

	padded := s + "   " + s
	offset := step % (len(s) + 3)
	return padded[offset : offset+maxLen]
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
