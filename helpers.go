package main

import "fmt"

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if maxLen > 3 && len(runes) > maxLen {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}

func marquee(s string, maxLen, step int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	padded := append(runes, []rune("   ")...)
	padded = append(padded, runes...)

	offset := step % (len(runes) + 3)
	return string(padded[offset : offset+maxLen])
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
