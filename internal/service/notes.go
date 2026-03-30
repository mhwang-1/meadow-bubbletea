package service

import "fmt"

// ReadNotes reads the global notes.md file.
// Returns an empty string (not an error) if the file does not exist.
func (s *Service) ReadNotes() (string, error) {
	return s.store.ReadNotes()
}

// WriteNotes writes the global notes.md file.
func (s *Service) WriteNotes(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.WriteNotes(content); err != nil {
		return fmt.Errorf("writing notes: %w", err)
	}
	return nil
}
