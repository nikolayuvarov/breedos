package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Self-update contract
//
// The running binary periodically checks for a sibling file named
// "<binary>.UPDATE". When it appears, the binary runs the candidate
// with the --self-check flag and expects the literal token "OK" on
// stdout. If that contract holds, the running binary renames itself
// to "<binary>.bak.<YYYYMMDDHHmmss>", renames the candidate into the
// running name, and exits with a non-zero status so systemd
// (Restart=on-failure) restarts the now-new binary.
//
// Any error during the self-check, rename, or swap is logged and the
// watcher continues polling — the running service does NOT go down
// on a bad candidate. The candidate file is left in place for
// inspection.

const (
	selfUpdateInterval    = 60 * time.Second
	selfCheckTimeout      = 10 * time.Second
	backupTimestampFormat = "20060102150405"
)

// startSelfUpdateWatcher launches a background goroutine watching for
// "<binary>.UPDATE" next to the running binary. The interval governs
// how often the file system is polled.
func startSelfUpdateWatcher(interval time.Duration) {
	binaryPath, err := os.Executable()
	if err != nil {
		log.Printf("self-update: cannot resolve own binary path: %v (watcher disabled)", err)
		return
	}
	if resolved, err := filepath.EvalSymlinks(binaryPath); err == nil {
		binaryPath = resolved
	}
	updatePath := binaryPath + ".UPDATE"
	log.Printf("self-update: watching for %s every %s", updatePath, interval)

	go selfUpdateLoop(binaryPath, updatePath, interval)
}

func selfUpdateLoop(binaryPath, updatePath string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if !updateFilePresent(updatePath) {
			continue
		}
		log.Printf("self-update: detected %s, validating", updatePath)
		if err := ensureExecutable(updatePath); err != nil {
			log.Printf("self-update: cannot ensure executable on %s: %v", updatePath, err)
			continue
		}
		if !selfCheckOK(updatePath) {
			log.Printf("self-update: %s --self-check did not return OK; skipping (file left in place for inspection)", updatePath)
			continue
		}
		log.Printf("self-update: self-check OK; swapping binaries")
		if err := performSwap(binaryPath, updatePath); err != nil {
			log.Printf("self-update: swap failed: %v", err)
			continue
		}
		log.Println("self-update: swap complete; exiting so systemd restarts the new binary")
		os.Exit(1)
	}
}

func updateFilePresent(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o100 != 0 {
		return nil
	}
	return os.Chmod(path, info.Mode().Perm()|0o111)
}

func selfCheckOK(updatePath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), selfCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, updatePath, "--self-check")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("self-update: self-check command error: %v", err)
		return false
	}
	return strings.TrimSpace(string(out)) == "OK"
}

func performSwap(currentPath, updatePath string) error {
	timestamp := time.Now().Format(backupTimestampFormat)
	backupPath := fmt.Sprintf("%s.bak.%s", currentPath, timestamp)
	if err := os.Rename(currentPath, backupPath); err != nil {
		return fmt.Errorf("backup rename %s -> %s: %w", currentPath, backupPath, err)
	}
	if err := os.Rename(updatePath, currentPath); err != nil {
		if rbErr := os.Rename(backupPath, currentPath); rbErr != nil {
			return fmt.Errorf("update rename %s -> %s: %w; rollback also failed: %v", updatePath, currentPath, err, rbErr)
		}
		return fmt.Errorf("update rename %s -> %s: %w (rolled back)", updatePath, currentPath, err)
	}
	return nil
}
