package cmd

import (
	"fmt"

	"github.com/jacksonfernando/a-kit/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of a-kit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("a-kit %s\n", version.Get())
	},
}
