package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hwang/meadow-bubbletea/internal/domain"
)

// Store provides read/write access to task list files on disk.
type Store struct {
	DataDir string
}

// NewStore creates a new Store rooted at the given data directory.
func NewStore(dataDir string) *Store {
	return &Store{DataDir: dataDir}
}

// TaskListDir returns the directory where task list files are stored.
func (s *Store) TaskListDir() string {
	return s.DataDir + "/active/tasklists"
}

// ReadTaskList reads a task list file from active/tasklists/{slug}.md,
// parses the YAML frontmatter for the name, and parses each non-empty
// line as a task.
func (s *Store) ReadTaskList(slug string) (*domain.TaskList, error) {
	path := filepath.Join(s.TaskListDir(), slug+".md")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading task list %q: %w", slug, err)
	}

	content := string(data)
	name, body, err := parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %q: %w", slug, err)
	}

	tl := &domain.TaskList{
		Name: name,
		Slug: slug,
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		task, err := domain.ParseTaskLine(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parsing task in %q: %w", slug, err)
		}
		tl.Tasks = append(tl.Tasks, task)
	}

	return tl, nil
}

// WriteTaskList writes a task list file using an exclusive lock.
// It creates directories if needed and uses atomic write (temp file + rename).
func (s *Store) WriteTaskList(tl *domain.TaskList) error {
	dir := s.TaskListDir()
	path := filepath.Join(dir, tl.Slug+".md")

	return WithLock(path, func() error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating tasklist directory: %w", err)
		}

		var b strings.Builder
		b.WriteString("---\n")
		b.WriteString("name: " + tl.Name + "\n")
		b.WriteString("---\n")

		for _, task := range tl.Tasks {
			b.WriteString(domain.FormatTaskLine(task) + "\n")
		}

		tmp, err := os.CreateTemp(dir, ".tasklist-*.md.tmp")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := tmp.WriteString(b.String()); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing temp file: %w", err)
		}

		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("closing temp file: %w", err)
		}

		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("renaming temp file: %w", err)
		}

		return nil
	})
}

// ListTaskLists reads all .md files in the tasklists directory and returns
// them sorted alphabetically by Name.
func (s *Store) ListTaskLists() ([]*domain.TaskList, error) {
	dir := s.TaskListDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing tasklists directory: %w", err)
	}

	var lists []*domain.TaskList
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		slug := strings.TrimSuffix(entry.Name(), ".md")
		tl, err := s.ReadTaskList(slug)
		if err != nil {
			return nil, err
		}
		lists = append(lists, tl)
	}

	sort.Slice(lists, func(i, j int) bool {
		return lists[i].Name < lists[j].Name
	})

	return lists, nil
}

// DeleteTaskList deletes the task list file for the given slug, using an
// exclusive lock.
func (s *Store) DeleteTaskList(slug string) error {
	path := filepath.Join(s.TaskListDir(), slug+".md")

	return WithLock(path, func() error {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("deleting task list %q: %w", slug, err)
		}
		return nil
	})
}

// ArchiveTaskList moves a task list file from active/tasklists/{slug}.md
// to archive/{YYYY}/{WW}/tasklists/{slug}.md using an exclusive lock.
func (s *Store) ArchiveTaskList(slug string, year, week int) error {
	srcPath := filepath.Join(s.TaskListDir(), slug+".md")

	return WithLock(srcPath, func() error {
		destDir := filepath.Join(s.ArchiveDir(year, week), "tasklists")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("creating archive tasklists directory: %w", err)
		}

		destPath := filepath.Join(destDir, slug+".md")

		// Try rename first (fast, same filesystem).
		if err := os.Rename(srcPath, destPath); err != nil {
			// Fallback: copy then remove (cross-filesystem).
			if err := copyFile(srcPath, destPath); err != nil {
				return fmt.Errorf("copying task list to archive: %w", err)
			}
			if err := os.Remove(srcPath); err != nil {
				return fmt.Errorf("removing source after copy: %w", err)
			}
		}

		return nil
	})
}

// copyFile copies a file from src to dest.
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// ListArchivedTaskLists scans archive directories for task list files and
// returns them grouped by year and week. Years and weeks are in descending order.
func (s *Store) ListArchivedTaskLists() (map[int]map[int][]*domain.TaskList, error) {
	archiveDir := filepath.Join(s.DataDir, "archive")

	result := make(map[int]map[int][]*domain.TaskList)

	yearEntries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("listing archive directory: %w", err)
	}

	for _, ye := range yearEntries {
		if !ye.IsDir() {
			continue
		}
		year, err := strconv.Atoi(ye.Name())
		if err != nil {
			continue
		}

		weekDir := filepath.Join(archiveDir, ye.Name())
		weekEntries, err := os.ReadDir(weekDir)
		if err != nil {
			continue
		}

		for _, we := range weekEntries {
			if !we.IsDir() {
				continue
			}
			week, err := strconv.Atoi(we.Name())
			if err != nil {
				continue
			}

			tasklistsDir := filepath.Join(weekDir, we.Name(), "tasklists")
			fileEntries, err := os.ReadDir(tasklistsDir)
			if err != nil {
				continue
			}

			var lists []*domain.TaskList
			for _, fe := range fileEntries {
				if fe.IsDir() || !strings.HasSuffix(fe.Name(), ".md") {
					continue
				}

				slug := strings.TrimSuffix(fe.Name(), ".md")
				tl, err := s.ReadArchivedTaskList(year, week, slug)
				if err != nil {
					continue
				}
				lists = append(lists, tl)
			}

			if len(lists) > 0 {
				sort.Slice(lists, func(i, j int) bool {
					return lists[i].Name < lists[j].Name
				})

				if result[year] == nil {
					result[year] = make(map[int][]*domain.TaskList)
				}
				result[year][week] = lists
			}
		}
	}

	return result, nil
}

// ReadArchivedTaskList reads a task list file from archive/{YYYY}/{WW}/tasklists/{slug}.md.
func (s *Store) ReadArchivedTaskList(year, week int, slug string) (*domain.TaskList, error) {
	path := filepath.Join(s.ArchiveDir(year, week), "tasklists", slug+".md")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading archived task list %q: %w", slug, err)
	}

	content := string(data)
	name, body, err := parseFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter in archived %q: %w", slug, err)
	}

	tl := &domain.TaskList{
		Name: name,
		Slug: slug,
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		task, err := domain.ParseTaskLine(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parsing task in archived %q: %w", slug, err)
		}
		tl.Tasks = append(tl.Tasks, task)
	}

	return tl, nil
}

// HasCompletedTasks checks whether any archived completed.md file contains
// a completed task with the given task list slug.
func (s *Store) HasCompletedTasks(slug string) (bool, error) {
	archiveDir := filepath.Join(s.DataDir, "archive")

	yearEntries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("listing archive directory: %w", err)
	}

	for _, ye := range yearEntries {
		if !ye.IsDir() {
			continue
		}
		year, err := strconv.Atoi(ye.Name())
		if err != nil {
			continue
		}

		weekDir := filepath.Join(archiveDir, ye.Name())
		weekEntries, err := os.ReadDir(weekDir)
		if err != nil {
			continue
		}

		for _, we := range weekEntries {
			if !we.IsDir() {
				continue
			}
			week, err := strconv.Atoi(we.Name())
			if err != nil {
				continue
			}

			completed, err := s.ReadCompleted(year, week)
			if err != nil {
				continue
			}

			for _, ct := range completed {
				if ct.TaskListSlug == slug {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// IsTaskListAssigned checks whether any non-archived daily timebox currently
// references the given task list slug.
func (s *Store) IsTaskListAssigned(slug string) (bool, error) {
	entries, err := os.ReadDir(s.TimeboxDir())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("listing timeboxes directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		date, err := domain.ParseDayFilename(entry.Name())
		if err != nil {
			continue
		}

		dt, err := s.ReadDailyTimeboxes(date)
		if err != nil {
			continue
		}

		for _, tb := range dt.Timeboxes {
			if tb.TaskListSlug == slug && tb.Status != domain.StatusArchived {
				return true, nil
			}
		}
	}

	return false, nil
}

// parseFrontmatter extracts the name from YAML frontmatter delimited by
// --- lines, and returns the name and the remaining body content.
func parseFrontmatter(content string) (name string, body string, err error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", fmt.Errorf("missing opening frontmatter delimiter")
	}

	// Find the closing ---
	rest := content[4:] // skip opening "---\n"
	closeIdx := strings.Index(rest, "\n---\n")
	if closeIdx < 0 {
		// Check if it ends with \n---
		if strings.HasSuffix(rest, "\n---") {
			closeIdx = len(rest) - 4
		} else {
			return "", "", fmt.Errorf("missing closing frontmatter delimiter")
		}
	}

	frontmatter := rest[:closeIdx]
	body = rest[closeIdx+4:] // skip "\n---\n"
	if strings.HasSuffix(rest, "\n---") && closeIdx == len(rest)-4 {
		body = ""
	}

	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		}
	}

	if name == "" {
		return "", "", fmt.Errorf("missing name in frontmatter")
	}

	return name, body, nil
}
