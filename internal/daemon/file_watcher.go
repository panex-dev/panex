package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const DefaultWatchDebounce = 50 * time.Millisecond

type FileChangeEvent struct {
	Paths      []string
	OccurredAt time.Time
}

type FileWatcher struct {
	root     string
	debounce time.Duration
	emit     func(FileChangeEvent)
	ready    chan struct{} // closed when Run enters its event loop; nil if unused
}

func NewFileWatcher(root string, debounce time.Duration, emit func(FileChangeEvent)) (*FileWatcher, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("watch root is required")
	}
	if debounce <= 0 {
		return nil, errors.New("debounce must be > 0")
	}
	if emit == nil {
		return nil, errors.New("emit callback is required")
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat watch root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("watch root must be a directory: %s", root)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve watch root %q: %w", root, err)
	}

	return &FileWatcher{
		root:     absRoot,
		debounce: debounce,
		emit:     emit,
	}, nil
}

func (w *FileWatcher) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}
	defer func() {
		_ = watcher.Close()
	}()

	if err := w.addDirectoryTree(watcher, w.root); err != nil {
		return err
	}
	if w.ready != nil {
		close(w.ready)
	}

	pending := make(map[string]struct{})
	var timer *time.Timer
	var timerCh <-chan time.Time

	flush := func() {
		if len(pending) == 0 {
			return
		}

		paths := make([]string, 0, len(pending))
		for path := range pending {
			paths = append(paths, path)
			delete(pending, path)
		}
		sort.Strings(paths)

		w.emit(FileChangeEvent{
			Paths:      paths,
			OccurredAt: time.Now().UTC(),
		})
	}

	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(w.debounce)
			timerCh = timer.C
			return
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		timer.Reset(w.debounce)
		timerCh = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			flush()
			return nil
		case err, ok := <-watcher.Errors:
			if !ok {
				flush()
				return nil
			}
			if timer != nil {
				timer.Stop()
			}
			return fmt.Errorf("watch filesystem: %w", err)
		case event, ok := <-watcher.Events:
			if !ok {
				flush()
				return nil
			}
			if !isRelevantFileEvent(event.Op) {
				continue
			}

			relPath, err := w.normalizePath(event.Name)
			if err == nil {
				pending[relPath] = struct{}{}
			}

			// New directories are not watched automatically by fsnotify.
			// On Windows there is also a race where files created inside a
			// new directory between MkdirAll and Add are missed entirely
			// by the platform watcher, so we synthesize pending entries
			// for any pre-existing children.
			if event.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					if err := w.addDirectoryTree(watcher, event.Name); err != nil {
						return err
					}
					w.synthesizeExistingChildren(event.Name, pending)
				}
			}

			resetTimer()
		case <-timerCh:
			flush()
			timerCh = nil
		}
	}
}

// synthesizeExistingChildren walks a freshly-watched directory and queues
// pending entries for any files already present. fsnotify on Windows can
// drop events for files created in a directory in the brief window between
// the directory's creation and its watch being registered; this closes that
// gap. Infrastructure dirs are skipped here just like in addDirectoryTree.
func (w *FileWatcher) synthesizeExistingChildren(root string, pending map[string]struct{}) {
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Best-effort: a child may have disappeared mid-walk. Continue.
			return nil //nolint:nilerr
		}
		if entry.IsDir() {
			if path != root && isInfrastructureDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, normErr := w.normalizePath(path); normErr == nil {
			pending[rel] = struct{}{}
		}
		return nil
	})
}

func (w *FileWatcher) addDirectoryTree(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if isInfrastructureDir(entry.Name()) {
			return filepath.SkipDir
		}

		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("add directory watch %q: %w", path, err)
		}

		return nil
	})
}

// isInfrastructureDir returns true for directories that should be excluded
// from file watching. This covers version control (.git), package manager
// caches (node_modules), IDE config (.vscode, .idea), and Panex output
// directories (.panex).
func isInfrastructureDir(name string) bool {
	return name == "node_modules" || strings.HasPrefix(name, ".")
}

func (w *FileWatcher) normalizePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	relPath, err := filepath.Rel(w.root, absPath)
	if err != nil {
		return "", err
	}
	if relPath == "." {
		return "", errors.New("root-level directory event")
	}
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return "", errors.New("event path is outside watch root")
	}

	clean := filepath.ToSlash(filepath.Clean(relPath))

	// Some platforms (notably Windows via ReadDirectoryChangesW) deliver an event
	// on the parent infrastructure directory itself when one of its children
	// changes, even though that directory was excluded from watching. Reject
	// any event whose first path segment is an infrastructure dir so callers
	// see consistent behavior across OSes.
	if first, _, _ := strings.Cut(clean, "/"); isInfrastructureDir(first) {
		return "", errors.New("event path is under an infrastructure directory")
	}

	return clean, nil
}

func isRelevantFileEvent(op fsnotify.Op) bool {
	return op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0
}
