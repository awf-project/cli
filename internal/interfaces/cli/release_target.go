package cli

import (
	"fmt"
	"strings"

	"github.com/awf-project/cli/pkg/registry"
)

type exactReleaseTarget struct {
	HasVersion bool
	Version    registry.Version
	Tag        string
}

func parseExactReleaseTarget(rawTarget string, optional bool) (exactReleaseTarget, error) {
	if rawTarget == "" && optional {
		return exactReleaseTarget{}, nil
	}

	version, err := registry.ParseVersion(rawTarget)
	if err != nil {
		return exactReleaseTarget{}, fmt.Errorf("invalid release version %q: %w", rawTarget, err)
	}

	return exactReleaseTarget{
		HasVersion: true,
		Version:    version,
		Tag:        registry.NormalizeTag(version.String()),
	}, nil
}

func parseInstallReleaseTarget(source string) (string, exactReleaseTarget, error) {
	repository := source
	rawTarget := ""
	hasVersionTarget := false

	if at := strings.LastIndex(source, "@"); at >= 0 {
		repository = source[:at]
		rawTarget = source[at+1:]
		hasVersionTarget = true
	} else if strings.Contains(source, ":") {
		return "", exactReleaseTarget{}, fmt.Errorf("owner/repo:version syntax is not supported; use owner/repo@version")
	}

	target, err := parseExactReleaseTarget(rawTarget, !hasVersionTarget)
	if err != nil {
		return "", exactReleaseTarget{}, err
	}

	return repository, target, nil
}

func selectExactRelease(
	releases []registry.Release,
	target exactReleaseTarget,
	includePrerelease bool,
) (registry.Release, error) {
	if target.HasVersion {
		for _, release := range releases {
			if registry.NormalizeTag(release.TagName) == target.Tag {
				return release, nil
			}
		}

		return registry.Release{}, fmt.Errorf("release version %s not found", target.Tag)
	}

	selected, found := registry.Release{}, false
	var selectedVersion registry.Version
	for _, release := range releases {
		if release.Prerelease && !includePrerelease {
			continue
		}

		version, err := registry.ParseVersion(registry.NormalizeTag(release.TagName))
		if err != nil {
			continue
		}

		if !found || version.Compare(selectedVersion) > 0 {
			selected = release
			selectedVersion = version
			found = true
		}
	}

	if !found {
		return registry.Release{}, fmt.Errorf("no stable releases found")
	}

	return selected, nil
}
