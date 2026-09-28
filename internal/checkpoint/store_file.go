package checkpoint

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrPath = errors.New("unsafe checkpoint path")

// WriteFile publishes a complete bundle at a previously absent path. It never
// replaces an existing directory entry (including a symlink). A same-directory
// exclusive temporary file is written, synced and closed before os.Link makes
// the final name visible; directory sync then makes that name durable. Linking
// instead of renaming deliberately fails closed on filesystems without hard
// links or directory Sync. Errors after Link may leave a complete, readable
// final artifact whose persistence is uncertain; a caller can ReadFile to
// reconcile. There is no mutable latest pointer.
func WriteFile(path string, sections []Section) ([32]byte, error) {
	return writeFile(path, sections, nil)
}

type publicationStage string

const (
	stageCreated   publicationStage = "created"
	stageWritten   publicationStage = "written"
	stageSynced    publicationStage = "synced"
	stageClosed    publicationStage = "closed"
	stageLinked    publicationStage = "linked"
	stageDirSynced publicationStage = "directory-synced"
)

func writeFile(path string, sections []Section, after func(publicationStage) error) ([32]byte, error) {
	return writeFileWithSync(path, sections, after, syncDirectory)
}

func writeFileWithSync(path string, sections []Section, after func(publicationStage) error, syncDir func(string) error) (digest [32]byte, err error) {
	dir, err := checkPath(path)
	if err != nil {
		return [32]byte{}, err
	}
	data, hash, err := Encode(sections)
	if err != nil {
		return [32]byte{}, err
	}
	f, err := os.CreateTemp(dir, ".checkpoint-")
	if err != nil {
		return [32]byte{}, err
	}
	temp := f.Name()
	defer func() {
		// Never remove the final name: another reader may already use it.
		// Persist removal of the temporary name even when publication fails
		// after the first directory sync. A failed cleanup sync leaves the
		// final complete but cleanup durability uncertain.
		if removeErr := os.Remove(temp); removeErr == nil {
			err = errors.Join(err, syncDir(dir))
		} else if !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
		if err != nil {
			digest = [32]byte{}
		}
	}()
	defer func() {
		if f != nil {
			err = errors.Join(err, f.Close())
		}
	}()
	step := func(stage publicationStage) error {
		if after != nil {
			return after(stage)
		}
		return nil
	}
	if err = step(stageCreated); err != nil {
		return [32]byte{}, err
	}
	var n int
	n, err = f.Write(data)
	if err != nil || n != len(data) {
		return [32]byte{}, errors.Join(err, io.ErrShortWrite)
	}
	if err = step(stageWritten); err != nil {
		return [32]byte{}, err
	}
	if err = f.Sync(); err != nil {
		return [32]byte{}, err
	}
	if err = step(stageSynced); err != nil {
		return [32]byte{}, err
	}
	if err = f.Close(); err != nil {
		f = nil
		return [32]byte{}, err
	}
	f = nil
	if err = step(stageClosed); err != nil {
		return [32]byte{}, err
	}
	// Link is an atomic no-clobber publication: no check-then-rename race.
	if err = os.Link(temp, path); err != nil {
		return [32]byte{}, err
	}
	if err = step(stageLinked); err != nil {
		return [32]byte{}, err
	}
	if err = syncDir(dir); err != nil {
		return [32]byte{}, err
	}
	if err = step(stageDirSynced); err != nil {
		return [32]byte{}, err
	}
	// The deferred cleanup unlinks the temp name and syncs that removal.
	return hash, nil
}

// ReadFile ignores unpublished temporary names and rejects nonregular paths.
// A successful result has been verified against the entire stored digest.
func ReadFile(path string) ([]Section, [32]byte, error) {
	if _, err := checkPath(path); err != nil {
		return nil, [32]byte{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if !info.Mode().IsRegular() {
		return nil, [32]byte{}, ErrPath
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, [32]byte{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, [32]byte{}, ErrPath
	}
	if opened.Size() < bundleHeaderSize+32 || opened.Size() > MaxBundleBytes {
		return nil, [32]byte{}, ErrBundle
	}
	// LimitReader prevents a growing file from causing an unbounded read;
	// Decode rejects a truncated bundle or an oversized result.
	data, err := io.ReadAll(io.LimitReader(f, MaxBundleBytes+1))
	if err != nil {
		return nil, [32]byte{}, err
	}
	return Decode(data)
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := f.Sync()
	return errors.Join(syncErr, f.Close())
}

func checkPath(path string) (string, error) {
	if path == "" || filepath.Clean(path) != path || strings.HasPrefix(filepath.Base(path), ".checkpoint-") ||
		filepath.Base(path) == "." || filepath.Base(path) == ".." {
		return "", ErrPath
	}
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		if component == ".." {
			return "", ErrPath
		}
	}
	if info, err := os.Lstat(path); err == nil && info.IsDir() {
		return "", ErrPath
	}
	dir := filepath.Dir(path)
	for current := dir; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() {
			return "", fmt.Errorf("%w: parent directory %s", ErrPath, current)
		}
		if current == "." || filepath.Dir(current) == current {
			break
		}
	}
	return dir, nil
}
