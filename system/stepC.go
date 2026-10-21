package system

import "database/sql"

type Session struct {
	ClaimID int
	status  string
	Note    string
}

func NewSession(claimID int) *Session {
	return &Session{ClaimID: claimID, status: "NEW", Note: "session created"}
}
func (s *Session) Status() string { return s.status }
func (s *Session) Confirm() error {
	if s.status != "NEW" {
		return ErrIllegalTransition
	}
	s.status = "CONFIRMED"
	s.Note = "anti-cheat passed"
	return nil
}
func (s *Session) Reject() error {
	if s.status != "NEW" {
		return ErrIllegalTransition
	}
	s.status = "REJECTED"
	s.Note = "anti-cheat rejected session"
	return nil
}

type AntiCheatScanner struct{}

func (AntiCheatScanner) Scan(forceDetection bool) error {
	if forceDetection {
		return ErrCheatDetected
	}
	return nil
}

type SessionLogger struct {
	db      *sql.DB
	scanner AntiCheatScanner
}

func NewSessionLogger(db *sql.DB) *SessionLogger { return &SessionLogger{db: db} }
func (l *SessionLogger) Finalize(claimID int, forceCheatDetection bool) error {
	s := NewSession(claimID)
	if err := l.scanner.Scan(forceCheatDetection); err != nil {
		_ = s.Reject()
		if saveErr := SaveSession(l.db, s); saveErr != nil {
			return saveErr
		}
		return err
	}
	if err := s.Confirm(); err != nil {
		return err
	}
	if err := SaveSession(l.db, s); err != nil {
		return err
	}
	return nil
}