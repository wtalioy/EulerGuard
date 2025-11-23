package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	maxLogSize = 100 * 1024 * 1024 // 100 MB
	maxBackups = 5
)

func rotateLogIfNeeded(logPath string) error {
	if logPath == "" {
		return nil
	}

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.Size() < maxLogSize {
		return nil
	}

	return rotateLog(logPath)
}

func rotateLog(logPath string) error {
	timestamp := time.Now().Format("20060102-150405")
	dir := filepath.Dir(logPath)
	base := filepath.Base(logPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	backupPath := filepath.Join(dir, fmt.Sprintf("%s-%s%s", name, timestamp, ext))

	if err := os.Rename(logPath, backupPath); err != nil {
		return fmt.Errorf("failed to rotate log: %w", err)
	}

	go cleanupOldLogs(dir, name, ext)

	return nil
}

func cleanupOldLogs(dir, baseName, ext string) {
	pattern := filepath.Join(dir, fmt.Sprintf("%s-*%s", baseName, ext))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	if len(matches) <= maxBackups {
		return
	}

	// Sort by modification time (oldest first) using efficient sort.Slice
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	
	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: match, modTime: info.ModTime()})
	}
	
	// Sort oldest first - O(n log n) instead of O(n²)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	
	// Delete oldest files (deleteCount is already bounded by len(files))
	deleteCount := len(files) - maxBackups
	for i := 0; i < deleteCount; i++ {
		_ = os.Remove(files[i].path)
	}
}
