package cmd

import "github.com/spf13/cobra"

const FlagStackName = "stack-name"

func registerStackName(cmd *cobra.Command) {
	cmd.Flags().StringP(
		FlagStackName, "s", "",
		"name of the target EListMan CloudFormation stack",
	)
}

func getStackName(cmd *cobra.Command) string {
	return getStringFlag(cmd, FlagStackName)
}

func getStringFlag(cmd *cobra.Command, flagName string) (value string) {
	if f := cmd.Flag(flagName); f != nil {
		value = f.Value.String()
	}
	return
}
