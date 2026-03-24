// Package pluginv1 contains the generated protobuf and gRPC code for the AWF plugin protocol.
// This file will be overwritten by `make proto-gen` (T005) once plugin.proto is defined (T004).
package pluginv1

import (
	_ "github.com/hashicorp/go-plugin"               // Required plugin framework for AWF plugins
	_ "google.golang.org/grpc"                       // Required gRPC framework for plugin communication
	_ "google.golang.org/protobuf/runtime/protoimpl" // Required protobuf runtime for message serialization
)
