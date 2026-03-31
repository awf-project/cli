// Package workflowpkg provides workflow pack installation, removal, and discovery.
// It mirrors the pluginmgr pattern: atomic temp+rename installs, per-pack state.json,
// and pkg/registry for all transport operations (download, checksum, extraction).
package workflowpkg
