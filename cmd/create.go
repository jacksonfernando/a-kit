package cmd

import (
	"fmt"
	"os"

	"github.com/jacksonfernando/a-kit/internal/scaffold"
	"github.com/spf13/cobra"
)

var moduleName string

var createCmd = &cobra.Command{
	Use:   "create [project-name]",
	Short: "Create a new Go project",
	Long: `Create scaffolds a new Go project following clean architecture.

The generated project includes:
  - Echo HTTP server with JWT middleware
  - GORM with MySQL repository layer
  - Clean separation: handler / service / repository
  - global/, models/, utils/, middlewares/ packages
  - A sample "example" module to get you started
  - Dockerfile, docker-compose.yml, Makefile, .env.example

Example:
  a-kit create my-service
  a-kit create my-service --module github.com/myorg/my-service`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		mod := moduleName
		if mod == "" {
			mod = projectName
		}

		opts := scaffold.Options{
			ProjectName: projectName,
			ModuleName:  mod,
		}

		fmt.Printf("🚀 Creating project %q (module: %s)...\n", projectName, mod)

		if err := scaffold.Generate(opts); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ Project %q created successfully!\n\n", projectName)
		fmt.Printf("  cd %s\n", projectName)
		fmt.Printf("  cp .env.example .env   # fill in your config\n")
		fmt.Printf("  go mod tidy\n")
		fmt.Printf("  go run main.go\n\n")
		return nil
	},
}

func init() {
	createCmd.Flags().StringVarP(&moduleName, "module", "m", "", "Go module name (default: project-name)")
}
