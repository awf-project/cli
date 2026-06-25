package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
)

const awfPluginSDKModulePath = "github.com/awf-project/cli"

var awfModuleVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)

type pluginInitFile struct {
	path    string
	content []byte
}

type pluginInitTemplateData struct {
	DistributionName   string
	RuntimeID          string
	Kind               string
	AWFModuleVersion   string
	LocalAWFModulePath string
	HasLocalAWFModule  bool
	Release            pluginInitReleaseTemplateData
}

type pluginInitReleaseTemplateData struct {
	Version       string
	ArchiveName   string
	ArchiveSuffix string
	ChecksumFile  string
	PackageDir    string
	BinaryName    string
	ManifestName  string
}

func newPluginInitReleaseTemplateData(options pluginInitOptions) pluginInitReleaseTemplateData {
	const version = "0.1.0"

	archiveSuffix := "$(GOOS)_$(GOARCH).tar.gz"
	return pluginInitReleaseTemplateData{
		Version:       version,
		ArchiveName:   fmt.Sprintf("%s_%s_%s", options.distributionName, version, archiveSuffix),
		ArchiveSuffix: archiveSuffix,
		ChecksumFile:  "checksums.txt",
		PackageDir:    "package",
		BinaryName:    options.distributionName,
		ManifestName:  "plugin.yaml",
	}
}

func renderPluginInitTemplate(options pluginInitOptions) ([]pluginInitFile, error) {
	localAWFModulePath, hasLocalAWFModule := currentAWFModulePath()
	awfModuleVersion, err := currentAWFModuleVersion(hasLocalAWFModule)
	if err != nil {
		return nil, err
	}
	data := pluginInitTemplateData{
		DistributionName:   options.distributionName,
		RuntimeID:          options.runtimeID,
		Kind:               options.kind,
		AWFModuleVersion:   awfModuleVersion,
		LocalAWFModulePath: localAWFModulePath,
		HasLocalAWFModule:  hasLocalAWFModule,
		Release:            newPluginInitReleaseTemplateData(options),
	}

	descriptors := operationPluginInitTemplates
	files := make([]pluginInitFile, 0, len(descriptors))
	for _, descriptor := range descriptors {
		tmpl, err := template.New(descriptor.path).Parse(descriptor.template)
		if err != nil {
			return nil, fmt.Errorf("parse plugin init template %s: %w", descriptor.path, err)
		}

		var content bytes.Buffer
		if err := tmpl.Execute(&content, data); err != nil {
			return nil, fmt.Errorf("render plugin init template %s: %w", descriptor.path, err)
		}

		files = append(files, pluginInitFile{
			path:    descriptor.path,
			content: content.Bytes(),
		})
	}

	return files, nil
}

func ensurePluginInitGoFlags() error {
	current := os.Getenv("GOFLAGS")
	for _, field := range strings.Fields(current) {
		if strings.HasPrefix(field, "-mod=") {
			return nil
		}
	}
	if current == "" {
		if err := os.Setenv("GOFLAGS", "-mod=mod"); err != nil {
			return fmt.Errorf("set plugin init go flags: %w", err)
		}
		return nil
	}
	if err := os.Setenv("GOFLAGS", current+" -mod=mod"); err != nil {
		return fmt.Errorf("set plugin init go flags: %w", err)
	}
	return nil
}

func currentAWFModuleVersion(hasLocalAWFModule bool) (string, error) {
	if hasLocalAWFModule {
		return "v0.0.0", nil
	}

	version := Version
	if version != "" && !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if awfModuleVersionPattern.MatchString(version) {
		return version, nil
	}

	return "", fmt.Errorf(
		"cannot scaffold plugin SDK dependency: run awf plugin init from an AWF source checkout or use a released awf binary with a semantic version",
	)
}

func currentAWFModulePath() (string, bool) {
	wd, err := os.Getwd()
	if err == nil {
		if root, ok := findAWFModuleRoot(wd); ok {
			return filepath.ToSlash(root), true
		}
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if ok {
		if root, found := findAWFModuleRoot(filepath.Dir(sourceFile)); found {
			return filepath.ToSlash(root), true
		}
	}

	return "", false
}

func findAWFModuleRoot(start string) (string, bool) {
	path := filepath.Clean(start)
	for {
		if isAWFModuleRoot(path) {
			return path, true
		}

		parent := filepath.Dir(path)
		if parent == path {
			return path, false
		}
		path = parent
	}
}

func isAWFModuleRoot(path string) bool {
	goModPath := filepath.Join(path, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "module "+awfPluginSDKModulePath+"\n")
}
