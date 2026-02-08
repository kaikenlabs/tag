package types

import "os"

// Standard file permission constants used throughout the codebase.
const (
	// DirMode is the default permission for created directories.
	DirMode os.FileMode = 0o755

	// DirModeRestricted is a more restrictive directory permission
	// used for directories within the user's .tag configuration.
	DirModeRestricted os.FileMode = 0o750

	// DirModePrivate is for directories that should only be accessible
	// by the owner (e.g., replay data containing potentially sensitive values).
	DirModePrivate os.FileMode = 0o700

	// FileMode is the default permission for created files.
	FileMode os.FileMode = 0o644

	// FileModePrivate is for files that should only be readable by
	// the owner (e.g., replay data with saved variable values).
	FileModePrivate os.FileMode = 0o600
)
