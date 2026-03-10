package build

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

var (
	scriptTagPattern       = regexp.MustCompile(`(?is)<script\b[^>]*>`)
	srcAttrPattern         = regexp.MustCompile(`(?i)\bsrc=(\"[^\"]*\"|'[^']*')`)
	chromeSimMarkerPattern = regexp.MustCompile(`(?i)<script[^>]*data-panex-chrome-sim[^>]*>`)
	closingHeadPattern     = regexp.MustCompile(`(?i)</head>`)
)

func discoverHTMLAssets(sourceDir string) ([]string, error) {
	assets := make([]string, 0, 4)

	err := filepath.WalkDir(sourceDir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isHTMLAsset(filePath) {
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
		return nil, fmt.Errorf("discover html assets in %q: %w", sourceDir, err)
	}

	sort.Strings(assets)
	return assets, nil
}

func (b *EsbuildBuilder) processHTMLAssets(assets []string) error {
	if len(assets) == 0 {
		return nil
	}

	if err := b.bundleChromeSimEntrypoint(); err != nil {
		return err
	}

	for _, relPath := range assets {
		sourcePath := filepath.Join(b.sourceDir, filepath.FromSlash(relPath))
		rawHTML, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read html asset %q: %w", sourcePath, err)
		}

		renderedHTML, err := b.renderHTMLAsset(relPath, string(rawHTML))
		if err != nil {
			return fmt.Errorf("render html asset %q: %w", sourcePath, err)
		}

		outPath := filepath.Join(b.outDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("create html output directory %q: %w", filepath.Dir(outPath), err)
		}
		if err := os.WriteFile(outPath, []byte(renderedHTML), 0o644); err != nil {
			return fmt.Errorf("write html asset %q: %w", outPath, err)
		}
	}

	return nil
}

func (b *EsbuildBuilder) bundleChromeSimEntrypoint() error {
	config := b.options.chromeSim
	if config == nil {
		return nil
	}
	if strings.TrimSpace(config.ModuleSourcePath) == "" {
		return errorsForBuildMessages("chrome-sim module source path is required")
	}

	result := api.Build(api.BuildOptions{
		AbsWorkingDir: b.sourceDir,
		EntryPoints:   []string{config.ModuleSourcePath},
		NodePaths:     config.NodePaths,
		Outfile:       filepath.Join(b.outDir, config.ModuleOutputName),
		Bundle:        true,
		Format:        api.FormatESModule,
		Platform:      api.PlatformBrowser,
		Write:         true,
		Sourcemap:     api.SourceMapLinked,
		LogLevel:      api.LogLevelSilent,
	})
	if len(result.Errors) == 0 {
		return nil
	}

	return errorsForBuildMessages(strings.Join(collectMessages(result.Errors), "; "))
}

func (b *EsbuildBuilder) renderHTMLAsset(relPath string, markup string) (string, error) {
	rewritten := rewriteScriptSources(markup)

	config := b.options.chromeSim
	if config == nil {
		return rewritten, nil
	}
	if chromeSimMarkerPattern.MatchString(rewritten) {
		return rewritten, nil
	}

	moduleURL := relativeModuleURL(relPath, config.ModuleOutputName)
	return injectChromeSimScriptTag(rewritten, chromeSimScriptTag(moduleURL, *config))
}

func rewriteScriptSources(markup string) string {
	return scriptTagPattern.ReplaceAllStringFunc(markup, func(tag string) string {
		srcAttr := srcAttrPattern.FindString(tag)
		if srcAttr == "" {
			return tag
		}

		quotedValue := srcAttr[len("src="):]
		if len(quotedValue) < 2 {
			return tag
		}

		quote := quotedValue[0]
		rawValue := quotedValue[1 : len(quotedValue)-1]
		rewrittenValue := rewriteBundledScriptPath(rawValue)
		if rewrittenValue == rawValue {
			return tag
		}

		replacement := fmt.Sprintf("src=%c%s%c", quote, html.EscapeString(rewrittenValue), quote)
		return strings.Replace(tag, srcAttr, replacement, 1)
	})
}

func rewriteBundledScriptPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" ||
		strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "data:") ||
		strings.HasPrefix(trimmed, "http:") ||
		strings.HasPrefix(trimmed, "https:") ||
		strings.HasPrefix(trimmed, "chrome-extension:") {
		return value
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return value
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return value
	}
	if !isBundleEntry(parsed.Path) {
		return value
	}

	ext := strings.ToLower(path.Ext(parsed.Path))
	if ext == ".js" {
		return value
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, path.Ext(parsed.Path)) + ".js"
	return parsed.String()
}

func relativeModuleURL(relHTMLPath, outputName string) string {
	baseDir := path.Dir(filepath.ToSlash(relHTMLPath))
	if baseDir == "." {
		baseDir = ""
	}

	modulePath, err := filepath.Rel(filepath.FromSlash(baseDir), filepath.FromSlash(outputName))
	if err != nil || modulePath == "" {
		modulePath = outputName
	}
	modulePath = filepath.ToSlash(modulePath)
	if !strings.HasPrefix(modulePath, ".") && !strings.HasPrefix(modulePath, "/") {
		modulePath = "./" + modulePath
	}
	return modulePath
}

func injectChromeSimScriptTag(markup, scriptTag string) (string, error) {
	indexes := closingHeadPattern.FindStringIndex(markup)
	if indexes == nil {
		return "", fmt.Errorf("html asset is missing a </head> tag")
	}

	return markup[:indexes[0]] + "  " + scriptTag + "\n" + markup[indexes[0]:], nil
}

func chromeSimScriptTag(moduleURL string, config ChromeSimInjectionOptions) string {
	attrs := []string{
		`type="module"`,
		fmt.Sprintf(`src="%s"`, html.EscapeString(moduleURL)),
		`data-panex-chrome-sim="1"`,
		fmt.Sprintf(`data-panex-ws="%s"`, html.EscapeString(config.DaemonURL)),
	}

	if token := strings.TrimSpace(config.AuthToken); token != "" {
		attrs = append(attrs, fmt.Sprintf(`data-panex-token="%s"`, html.EscapeString(token)))
	}
	if extensionID := strings.TrimSpace(config.ExtensionID); extensionID != "" {
		attrs = append(attrs, fmt.Sprintf(`data-panex-extension-id="%s"`, html.EscapeString(extensionID)))
	}

	return "<script " + strings.Join(attrs, " ") + "></script>"
}

func errorsForBuildMessages(message string) error {
	return fmt.Errorf("bundle chrome-sim entrypoint: %s", message)
}
