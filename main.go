// Copyright © 2023 Mike Bland <mbland@acm.org>.
// See LICENSE.txt for details.

package main

import (
	"os"

	"github.com/mbland/elistman/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
