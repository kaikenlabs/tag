package fileutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// uniqueAttempts bounds the retry loop that finds an unused temp name.
const uniqueAttempts = 100

var errNoUniqueName = errors.New("cannot find an unused temporary name")

func uniqueSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// WriteFileAtomic writes data to path by filling a uniquely named temporary
// file in the same directory, flushing it to stable storage, and renaming it
// over path.
//
// The name being unique is the whole point: writers that share one fixed temp
// path can rename each other's half-written files into place, so two concurrent
// saves of the same key corrupt or lose the target. The temp file is created
// with perm through O_CREATE rather than chmod'd afterwards, so the process
// umask applies exactly as it does to os.WriteFile.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}

	for range uniqueAttempts {
		suffix, err := uniqueSuffix()
		if err != nil {
			return err
		}

		tmp := filepath.Join(dir, base+".tmp-"+suffix)
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}

		if err := writeAndSync(f, data); err != nil {
			os.Remove(tmp)
			return err
		}

		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp)
			return err
		}
		return nil
	}

	return errNoUniqueName
}

func writeAndSync(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	// Rename only promises the directory entry swaps atomically, not that the
	// bytes reached disk first; without this a crash can publish a truncated
	// or zero-length file.
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// MkdirUnique creates a uniquely named directory under dir and returns its
// path. It is os.MkdirTemp except that perm goes through os.Mkdir, so the
// process umask applies; os.MkdirTemp hardcodes 0700 and a follow-up chmod
// would ignore the umask and widen the directory under a restrictive one.
func MkdirUnique(dir, prefix string, perm os.FileMode) (string, error) {
	for range uniqueAttempts {
		suffix, err := uniqueSuffix()
		if err != nil {
			return "", err
		}

		path := filepath.Join(dir, prefix+suffix)
		err = os.Mkdir(path, perm)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path, nil
	}

	return "", errNoUniqueName
}
