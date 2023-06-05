// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mbland/elistman/events"
	"github.com/spf13/cobra"
)

const importDescription = `` +
	`Subscribes a list of email addresses directly without verification

Reads the list of addresses from standard input, one address per line.

This is useful for importing a list of existing subscribers from a previous
system. Will not import addresses that fail validation, and will not override
records for existing verified subscribers.
`

func init() {
	rootCmd.AddCommand(newImportCmd(NewEListManLambda))
}

func newImportCmd(newFunc EListManFactoryFunc) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "import",
		Short: "Import existing subscribers from another system",
		Long:  importDescription,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			return importAddresses(cmd, newFunc, getStackName(cmd))
		},
	}
	registerStackName(cmd)
	cmd.MarkFlagRequired(FlagStackName)
	return
}

func importAddresses(
	cmd *cobra.Command, newFunc EListManFactoryFunc, stackName string,
) (err error) {
	cmd.SilenceUsage = true
	var addresses []string
	var elistmanFunc EListManFunc

	if addresses, err = readLines(cmd.InOrStdin()); err != nil {
		err = fmt.Errorf("failed to read email addresses from stdin: %w", err)
		return
	} else if elistmanFunc, err = newFunc(stackName); err != nil {
		return
	}

	ctx := context.Background()
	evt := &events.CommandLineEvent{
		EListManCommand: events.CommandLineImportEvent,
		Import:          &events.ImportEvent{Addresses: addresses},
	}
	var response *events.ImportResponse

	if err = elistmanFunc.Invoke(ctx, evt, &response); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	cmd.Print(importSuccessMessage(response.NumImported, len(addresses)))
	err = errorIfImportFailures(response.Failures)
	return
}

func readLines(stdin io.Reader) (lines []string, err error) {
	lines = make([]string, 0, 100)
	scanner := bufio.NewScanner(stdin)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	return
}

func importSuccessMessage(numImported, total int) string {
	if numImported == 1 {
		return "Successfully imported one address.\n"
	}
	const msgFmt = "Successfully imported %d of %d addresses.\n"
	return fmt.Sprintf(msgFmt, numImported, total)
}

func errorIfImportFailures(failures []string) error {
	if len(failures) == 0 {
		return nil
	} else if len(failures) == 1 {
		return fmt.Errorf("failed to import %s", failures[0])
	}
	const errFmt = "failed to import the following %d addresses:\n  %s"
	return fmt.Errorf(errFmt, len(failures), strings.Join(failures, "\n  "))
}
