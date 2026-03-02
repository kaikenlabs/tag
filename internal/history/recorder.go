package history

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/writer"
)

// Recorder accumulates FileEntry records for a single generation.
// It tracks which files have already been snapshotted so that hash_before
// is only captured once per file per generation (first-touch semantics),
// which is correct when a bundle runs multiple generators against the same file.
type Recorder struct {
	tagDir      string
	genID       string
	snapshotted map[string]bool // files for which hash_before has been captured
	entries     map[string]*FileEntry
}

// NewRecorder creates a Recorder that will store backups under tagDir.
func NewRecorder(tagDir string) *Recorder {
	return &Recorder{
		tagDir:      tagDir,
		genID:       newGenID(),
		snapshotted: make(map[string]bool),
		entries:     make(map[string]*FileEntry),
	}
}

// RecordCreate records a file that was newly created (no prior content).
// Should be called after the file has been written.
func (r *Recorder) RecordCreate(path, hashAfter string) {
	if e, exists := r.entries[path]; exists {
		// File was already recorded in this generation; update hash_after only.
		e.HashAfter = hashAfter
		return
	}
	r.entries[path] = &FileEntry{
		Path:       path,
		Action:     ActionCreate,
		HashBefore: nil,
		HashAfter:  hashAfter,
	}
}

// RecordModify records a file that was modified (inject or append).
// Should be called after the file has been written.
// hashBefore is the hash before this generation touched the file (first-touch only).
func (r *Recorder) RecordModify(path, action, hashBefore, hashAfter string) {
	if e, exists := r.entries[path]; exists {
		// Already recorded in this generation; only update hash_after.
		e.Action = action
		e.HashAfter = hashAfter
		return
	}
	hb := hashBefore
	r.entries[path] = &FileEntry{
		Path:       path,
		Action:     action,
		HashBefore: &hb,
		HashAfter:  hashAfter,
	}
}

// BackupDir returns the path where backups for this generation are stored.
func (r *Recorder) BackupDir() string {
	return filepath.Join(r.tagDir, types.HistoryBackupsDir, r.genID)
}

// Build returns a completed Generation with all recorded entries.
func (r *Recorder) Build(template, command string) Generation {
	files := make([]FileEntry, 0, len(r.entries))
	for _, e := range r.entries {
		files = append(files, *e)
	}
	// Sort by path for deterministic manifest output.
	slices.SortFunc(files, func(a, b FileEntry) int {
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	})
	return Generation{
		ID:        r.genID,
		Timestamp: time.Now().UTC(),
		Template:  template,
		Command:   command,
		Files:     files,
	}
}

// GenID returns the generation ID assigned to this recorder.
func (r *Recorder) GenID() string { return r.genID }

// newGenID generates a generation ID in the form gen_<unix>_<6hex>.
func newGenID() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a deterministic suffix on error (extremely unlikely).
		return fmt.Sprintf("gen_%d_000000", time.Now().Unix())
	}
	return fmt.Sprintf("gen_%d_%x", time.Now().Unix(), b)
}

// RecordingFileWriter wraps a writer.FileWriter to record file operations
// via a Recorder. It implements writer.FileWriter.
type RecordingFileWriter struct {
	inner  writer.FileWriter
	rec    *Recorder
	tagDir string
}

// NewRecordingFileWriter creates a RecordingFileWriter decorator.
func NewRecordingFileWriter(inner writer.FileWriter, rec *Recorder) *RecordingFileWriter {
	return &RecordingFileWriter{inner: inner, rec: rec, tagDir: rec.tagDir}
}

// Ensure RecordingFileWriter satisfies writer.FileWriter at compile time.
var _ writer.FileWriter = (*RecordingFileWriter)(nil)

// WriteFile records a "create" or "overwrite" operation and delegates to the inner writer.
// When the target file already exists (overwrite), snapshotBefore backs it up and captures
// hash_before so that undo can restore it.
func (w *RecordingFileWriter) WriteFile(name string, data []byte, perm fs.FileMode) error {
	hashBefore, existed, err := w.snapshotBefore(name)
	if err != nil {
		return err
	}

	if writeErr := w.inner.WriteFile(name, data, perm); writeErr != nil {
		return writeErr
	}

	hashAfter, err := HashFile(name)
	if err != nil {
		return fmt.Errorf("hash after write %s: %w", name, err)
	}

	if !existed {
		w.rec.RecordCreate(name, hashAfter)
	} else {
		w.rec.RecordModify(name, ActionOverwrite, hashBefore, hashAfter)
	}
	return nil
}

// AppendFile records an "append" operation. If the file does not exist before
// the append, it records a "create" action (hash_before=nil).
func (w *RecordingFileWriter) AppendFile(name string, data []byte) error {
	hashBefore, existed, err := w.snapshotBefore(name)
	if err != nil {
		return err
	}

	if appendErr := w.inner.AppendFile(name, data); appendErr != nil {
		return appendErr
	}

	hashAfter, err := HashFile(name)
	if err != nil {
		return fmt.Errorf("hash after append %s: %w", name, err)
	}

	if !existed {
		w.rec.RecordCreate(name, hashAfter)
	} else {
		w.rec.RecordModify(name, ActionAppend, hashBefore, hashAfter)
	}
	return nil
}

// InjectIntoFile records an "inject" operation.
func (w *RecordingFileWriter) InjectIntoFile(name string, data []byte, inject writer.Inject) error {
	hashBefore, existed, err := w.snapshotBefore(name)
	if err != nil {
		return err
	}

	if injectErr := w.inner.InjectIntoFile(name, data, inject); injectErr != nil {
		return injectErr
	}

	hashAfter, err := HashFile(name)
	if err != nil {
		return fmt.Errorf("hash after inject %s: %w", name, err)
	}

	if !existed {
		w.rec.RecordCreate(name, hashAfter)
	} else {
		w.rec.RecordModify(name, ActionInject, hashBefore, hashAfter)
	}
	return nil
}

// snapshotBefore captures the hash of name (if it exists) and creates a backup.
// Returns (hash, existed, error). Uses first-touch semantics: if the file has
// already been snapshotted in this generation, the backup is not recreated.
func (w *RecordingFileWriter) snapshotBefore(name string) (hashBefore string, existed bool, err error) {
	_, statErr := os.Stat(name)
	if errors.Is(statErr, fs.ErrNotExist) {
		return "", false, nil
	}
	if statErr != nil {
		return "", false, fmt.Errorf("stat %s: %w", name, statErr)
	}

	if !w.rec.snapshotted[name] {
		// First touch: capture hash and create backup.
		h, err := HashFile(name)
		if err != nil {
			return "", true, err
		}
		if err := w.backupFile(name); err != nil {
			return "", true, err
		}
		w.rec.snapshotted[name] = true
		return h, true, nil
	}

	// Subsequent touch: hash_before was already captured; return empty string
	// so the caller calls RecordModify with an empty hashBefore, which
	// RecordModify ignores (it only updates hash_after for subsequent touches).
	return "", true, nil
}

// backupFile copies name to the generation backup directory.
func (w *RecordingFileWriter) backupFile(name string) error {
	backupPath := filepath.Join(w.rec.BackupDir(), name)
	if err := os.MkdirAll(filepath.Dir(backupPath), types.DirMode); err != nil {
		return fmt.Errorf("create backup dir for %s: %w", name, err)
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read for backup %s: %w", name, err)
	}

	info, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("stat for backup %s: %w", name, err)
	}

	if err := os.WriteFile(backupPath, data, info.Mode()); err != nil {
		return fmt.Errorf("write backup %s: %w", backupPath, err)
	}
	return nil
}
