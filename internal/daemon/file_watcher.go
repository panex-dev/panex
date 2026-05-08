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

	// watchedDirs tracks every directory currently registered with fsnotify.
	// The tree-sync ticker compares the on-disk tree against this set to
	// discover subdirectories whose Create event fsnotify never delivered —
	// a known Windows gap where ReadDirectoryChangesW races against
	// rapidly-created children.
	watchedDirs := make(map[string]struct{})
	if err := w.addDirectoryTree(watcher, w.root, watchedDirs); err != nil {
		return err
	}
	if w.ready != nil {
		close(w.ready)
	}

	pending := make(map[string]struct{})
	// recentlyAddedDirs tracks subdirectories the watcher attached during
	// the run, with a remaining re-walk budget. Each tree-sync tick
	// re-walks these dirs and synthesizes pending entries for any files
	// they contain, closing the Windows window where Write events on a
	// freshly-attached watch are silently dropped.
	recentlyAddedDirs := make(map[string]int)
	const recentDirRewalkBudget = 4 // 4 ticks * 500ms ≈ 2s coverage
	const treeSyncInterval = 500 * time.Millisecond
	var timer *time.Timer
	var timerCh <-chan time.Time

	// Always-on tree-sync ticker. It runs regardless of fsnotify event
	// delivery so the "missed Create event" case on Windows still
	// converges within one tick.
	treeSyncTicker := time.NewTicker(treeSyncInterval)
	defer treeSyncTicker.Stop()

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

	syncTree := func() {
		_ = filepath.WalkDir(w.root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil //nolint:nilerr // best-effort: directories can appear/disappear mid-walk
			}
			if !entry.IsDir() {
				return nil
			}
			if path != w.root && isInfrastructureDir(entry.Name()) {
				return filepath.SkipDir
			}
			if _, ok := watchedDirs[path]; ok {
				return nil
			}
			if err := watcher.Add(path); err == nil {
				watchedDirs[path] = struct{}{}
				recentlyAddedDirs[path] = recentDirRewalkBudget
			}
			return nil
		})

		for dir, budget := range recentlyAddedDirs {
			w.synthesizeExistingChildren(dir, pending)
			if budget <= 1 {
				delete(recentlyAddedDirs, dir)
			} else {
				recentlyAddedDirs[dir] = budget - 1
			}
		}
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
			if err == nil && !isInfrastructurePath(relPath) {
				pending[relPath] = struct{}{}
			}

			// Attach new directories eagerly on Create so we don't wait up to
			// one tick for the tree-sync pass to find them. Skip infra dirs
			// to avoid wasteful walks under .git / node_modules / .panex.
			if event.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					if !isInfrastructureDir(filepath.Base(event.Name)) {
						if err := w.addDirectoryTree(watcher, event.Name, watchedDirs); err != nil {
							return err
						}
						w.synthesizeExistingChildren(event.Name, pending)
						recentlyAddedDirs[event.Name] = recentDirRewalkBudget
					}
				}
			}

			resetTimer()
		case <-treeSyncTicker.C:
			before := len(pending)
			syncTree()
			if len(pending) > before {
				resetTimer()
			}
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

func (w *FileWatcher) addDirectoryTree(watcher *fsnotify.Watcher, root string, watchedDirs map[string]struct{}) error {
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
		if _, ok := watchedDirs[path]; ok {
			return nil
		}

		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("add directory watch %q: %w", path, err)
		}
		watchedDirs[path] = struct{}{}

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

// isInfrastructurePath returns true when a normalized relative path falls
// inside an infrastructure directory. On Windows, the root-level watcher can
// report events for unwatched subdirectories when their contents change.
func isInfrastructurePath(relPath string) bool {
	top, _, _ := strings.Cut(relPath, "/")
	return isInfrastructureDir(top)
}

func isRelevantFileEvent(op fsnotify.Op) bool {
	return op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0
}
