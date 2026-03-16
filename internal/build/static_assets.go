package build

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func discoverStaticAssets(sourceDir string) ([]string, error) {
	assets := make([]string, 0, 8)

	err := filepath.WalkDir(sourceDir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if isInfrastructureDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isBundleEntry(filePath) || isHTMLAsset(filePath) {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return err
		}
		assets = append(assets, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover static assets in %q: %w", sourceDir, err)
	}

	sort.Strings(assets)
	return assets, nil
}

func (b *EsbuildBuilder) processStaticAssets(assets []string) error {
	for _, relPath := range assets {
		sourcePath := filepath.Join(b.sourceDir, filepath.FromSlash(relPath))
		value, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read static asset %q: %w", sourcePath, err)
		}

		outPath := filepath.Join(b.outDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("create static asset output directory %q: %w", filepath.Dir(outPath), err)
		}
		if err := os.WriteFile(outPath, value, 0o644); err != nil {
			return fmt.Errorf("write static asset %q: %w", outPath, err)
		}
	}

	return nil
}

func isHTMLAsset(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".html")
}
