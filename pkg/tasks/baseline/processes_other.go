//go:build !linux
// +build !linux

package tasks

import "github.com/controlplaneio/sandbox-probe/pkg/models"

func getRunningProcessCommandLinux(_ int) (*models.Process, error) { return nil, nil }
func getRunningParentProcessLinux(_ int) (*models.Process, error)  { return nil, nil }
func getRunningProcessesCommandsLinux() ([]*models.Process, error) { return nil, nil }
