//go:build windows

package main

import (
	"context"

	"github.com/ChamberZ40/magpie/config"
)

func runRunAsUserStartupChecks(_ context.Context, _ *config.Config) error {
	return nil
}
