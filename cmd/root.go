package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "a-kit",
	Short: "A scaffolding CLI for Go projects",
	Long:  `a-kit is a CLI tool that scaffolds new Go projects with a clean architecture structure.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(createCmd)
}
