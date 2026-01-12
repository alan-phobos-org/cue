package auth

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"sync"
	"time"
)

// SecurityEvent represents a logged security event.
type SecurityEvent struct {
	Timestamp  string `json:"ts"`
	Event      string `json:"event"`
	UserCN     string `json:"user,omitempty"`
	AuthMethod string `json:"method,omitempty"`
	TokenID    string `json:"token_id,omitempty"`
	SourceIP   string `json:"ip,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Details    string `json:"details,omitempty"`
}

// FileSecurityLogger writes security events to a file in JSON Lines format.
type FileSecurityLogger struct {
	mu     sync.Mutex
	writer io.Writer
	file   *os.File
}

// secretPattern matches strings that might be secrets (long base64, hex strings).
var secretPattern = regexp.MustCompile(`(eyJ[A-Za-z0-9_-]{20,}|[A-Fa-f0-9]{32,}|[A-Za-z0-9+/]{32,}={0,2})`)

// NewFileSecurityLogger creates a new logger that writes to the given file path.
func NewFileSecurityLogger(path string) (*FileSecurityLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &FileSecurityLogger{file: f, writer: f}, nil
}

// NewSecurityLogger creates a logger that writes to an io.Writer.
// Useful for testing.
func NewSecurityLogger(w io.Writer) *FileSecurityLogger {
	return &FileSecurityLogger{writer: w}
}

// Close closes the log file if one was opened.
func (l *FileSecurityLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// LogAuthSuccess logs a successful authentication event.
func (l *FileSecurityLogger) LogAuthSuccess(user *UserContext, sourceIP string) {
	l.log(SecurityEvent{
		Event:      "auth_success",
		UserCN:     user.CN,
		AuthMethod: user.AuthMethod,
		TokenID:    user.TokenID,
		SourceIP:   sourceIP,
	})
}

// LogAuthFailure logs a failed authentication attempt.
func (l *FileSecurityLogger) LogAuthFailure(reason, details, sourceIP string) {
	l.log(SecurityEvent{
		Event:    "auth_failure",
		Reason:   reason,
		Details:  sanitize(details),
		SourceIP: sourceIP,
	})
}

// LogTokenCreated logs when a new API token is created.
func (l *FileSecurityLogger) LogTokenCreated(userCN, tokenID, tokenName, expiresAt, sourceIP string) {
	l.log(SecurityEvent{
		Event:    "token_created",
		UserCN:   userCN,
		TokenID:  tokenID,
		Details:  "name=" + sanitize(tokenName) + ", expires=" + expiresAt,
		SourceIP: sourceIP,
	})
}

// LogTokenRevoked logs when a token is deleted/revoked.
func (l *FileSecurityLogger) LogTokenRevoked(userCN, tokenID, sourceIP string) {
	l.log(SecurityEvent{
		Event:    "token_revoked",
		UserCN:   userCN,
		TokenID:  tokenID,
		SourceIP: sourceIP,
	})
}

// LogServerStart logs server startup.
func (l *FileSecurityLogger) LogServerStart(mode, caFile string) {
	details := "mode=" + mode
	if caFile != "" {
		details += ", ca=" + caFile
	}
	l.log(SecurityEvent{
		Event:   "server_start",
		Details: details,
	})
}

// LogServerStop logs server shutdown.
func (l *FileSecurityLogger) LogServerStop(reason string) {
	l.log(SecurityEvent{
		Event:  "server_stop",
		Reason: reason,
	})
}

func (l *FileSecurityLogger) log(event SecurityEvent) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339)

	l.mu.Lock()
	defer l.mu.Unlock()

	json.NewEncoder(l.writer).Encode(event)
}

// sanitize removes potential secrets and truncates long strings.
func sanitize(s string) string {
	// Truncate long strings
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	// Mask potential secrets
	s = secretPattern.ReplaceAllString(s, "[REDACTED]")
	return s
}

// Reopen reopens the log file (for log rotation via SIGHUP).
func (l *FileSecurityLogger) Reopen() error {
	if l.file == nil {
		return nil
	}

	path := l.file.Name()

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.file.Close(); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	l.file = f
	l.writer = f
	return nil
}
