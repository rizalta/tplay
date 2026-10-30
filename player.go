package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

type playerTracker struct {
	sockPath string
	stopChan chan struct{}
	doneChan chan struct{}
	pos      float64
	dur      float64
	mu       sync.Mutex
}

func newPlayerTracker(sockPath string) *playerTracker {
	t := &playerTracker{
		sockPath: sockPath,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}

	go t.run()
	return t
}

func (t *playerTracker) run() {
	defer close(t.doneChan)

	var conn net.Conn
	var err error

	for range 40 {
		select {
		case <-t.stopChan:
			return
		default:
		}
		conn, err = net.DialTimeout("unix", t.sockPath, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if conn == nil {
		return
	}
	defer func() { _ = conn.Close() }()

	go func() {
		<-t.stopChan
		_ = conn.Close()
	}()

	_, _ = conn.Write([]byte(`{"command": ["observe_property", 1, "time-pos"]}` + "\n"))
	_, _ = conn.Write([]byte(`{"command": ["observe_property", 2, "duration"]}` + "\n"))

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()

		var msg struct {
			Event string          `json:"event"`
			Name  string          `json:"name"`
			Data  json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(line, &msg); err == nil && msg.Event == "property-change" {
			t.mu.Lock()
			if msg.Name == "time-pos" {
				var val float64
				if json.Unmarshal(msg.Data, &val) == nil {
					t.pos = val
				}
			}
			if msg.Name == "duration" {
				var val float64
				if json.Unmarshal(msg.Data, &val) == nil {
					t.dur = val
				}
			}
			t.mu.Unlock()
		}
	}
}

func (t *playerTracker) stop() (float64, float64) {
	if t == nil {
		return 0, 0
	}

	close(t.stopChan)
	<-t.doneChan
	_ = os.Remove(t.sockPath)

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.pos, t.dur
}

func resolveMPV() (string, error) {
	cmds := []string{
		"mpv",
		"/opt/homebrew/bin/mpv",
		"/usr/local/bin/mpv",
	}

	for _, cmd := range cmds {
		if path, err := exec.LookPath(cmd); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("mpv not found: please install mpv (https://mpv.io)")
}

func mpvCmd(playerPath, streamURL, sockPath string, resumeSec float64) *exec.Cmd {
	var args []string

	if resumeSec > 0 {
		args = append(args, fmt.Sprintf("--start=%.2f", resumeSec))
	}

	if sockPath != "" {
		args = append(args, "--input-ipc-server="+sockPath)
	}

	args = append(args, streamURL)
	return exec.Command(playerPath, args...)
}
