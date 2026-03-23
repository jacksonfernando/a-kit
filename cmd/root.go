package cmd

import (
	"fmt"
	"os"

	"github.com/jacksonfernando/a-kit/internal/version"
	"github.com/spf13/cobra"
)

const bannerFmt = `
  ██████╗       ██╗  ██╗██╗████████╗
  ██╔══██╗      ██║ ██╔╝██║╚══██╔══╝
  ███████║█████╗█████╔╝ ██║   ██║
  ██╔══██║╚════╝██╔═██╗ ██║   ██║
  ██║  ██║      ██║  ██╗██║   ██║
  ╚═╝  ╚═╝      ╚═╝  ╚═╝╚═╝   ╚═╝

  Go project scaffolding CLI   %s
`

var rootCmd = &cobra.Command{
	Use:   "a-kit",
	Short: "A scaffolding CLI for Go projects",
	Long:  `a-kit is a CLI tool that scaffolds new Go projects with a clean architecture structure.`,
}

func Execute() {
	fmt.Printf(bannerFmt, version.Get())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(versionCmd)
}
