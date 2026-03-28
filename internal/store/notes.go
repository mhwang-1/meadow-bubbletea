package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadNotes reads the notes.md file from the data directory root.
// Returns an empty string (not an error) if the file does not exist.
func (s *Store) ReadNotes() (string, error) {
	path := filepath.Join(s.DataDir, "notes.md")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading notes: %w", err)
	}

	return string(data), nil
}

// WriteNotes writes the notes.md file using an exclusive lock and atomic write.
func (s *Store) WriteNotes(content string) error {
	path := filepath.Join(s.DataDir, "notes.md")

	return WithLock(path, func() error {
		tmp, err := os.CreateTemp(s.DataDir, ".notes-*.md.tmp")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := tmp.WriteString(content); err != nil {
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
