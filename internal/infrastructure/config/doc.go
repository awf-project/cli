// Package config provides project configuration file loading and parsing.
//
// The config package is responsible for reading .awf/config.yaml files
// and providing pre-populated input values for workflow execution.
//
// Configuration Loading:
//
// The primary entry point is YAMLConfigLoader which reads configuration
// from the project's .awf/config.yaml file:
//
//	loader := config.NewYAMLConfigLoader(configPath)
//	cfg, err := loader.Load()
//	if err != nil {
//	    // Handle config error
//	}
//	// Use cfg.Inputs for workflow input defaults
//
// Merge Priority:
//
// Config file values have the lowest priority and are overridden by
// CLI --input flags. This allows config to provide defaults while
// still allowing runtime customization.
package config
