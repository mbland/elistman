// Copyright © 2023 Mike Bland <mbland@acm.org>.
// See LICENSE.txt for details.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "elistman",
	Version: "v0.1.0",
	Short: "Mailing list system providing address validation " +
		"and unsubscribe URIs",
	Long: ``,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
