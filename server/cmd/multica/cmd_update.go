package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Show how this local build is updated",
	RunE:  runUpdate,
}

func runUpdate(_ *cobra.Command, _ []string) error {
	return errors.New(cli.LocalReleaseUpdateMessage)
}
