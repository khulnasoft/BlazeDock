package cmd

import (
	"fmt"

	"github.com/khulnasoft/blazedock/pkg/blazedock"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of this blazedock build",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(blazedock.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
