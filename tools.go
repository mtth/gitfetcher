//go:build tools

package main

import (
	_ "github.com/charmbracelet/bubbles"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/dmarkham/enumer"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
