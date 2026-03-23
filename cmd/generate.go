package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacksonfernando/a-kit/internal/proto"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate [module-name]",
	Short: "Generate module code from protobuf definitions in api/",
	Long: `Generate reads .proto files from the api/ directory and regenerates
Go code for each module: handler, service, repository, interfaces, mocks, and models.

Must be run inside a project directory (where go.mod lives).

Examples:
  a-kit generate              # regenerate all modules from api/*.proto
  a-kit generate example      # regenerate only example from api/example.proto`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		modulePath, err := proto.ReadModuleName(cwd)
		if err != nil {
			return fmt.Errorf("failed to read go.mod: %w\n(make sure you are inside a project directory)", err)
		}

		apiDir := filepath.Join(cwd, "api")
		if _, err := os.Stat(apiDir); os.IsNotExist(err) {
			return fmt.Errorf("api/ directory not found — create api/<module>.proto files first")
		}

		var protoFiles []string
		if len(args) == 1 {
			protoFiles = []string{filepath.Join(apiDir, args[0]+".proto")}
		}

		if len(protoFiles) == 0 {
			entries, err := os.ReadDir(apiDir)
			if err != nil {
				return fmt.Errorf("reading api/: %w", err)
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".proto") {
					protoFiles = append(protoFiles, filepath.Join(apiDir, e.Name()))
				}
			}
		}

		if len(protoFiles) == 0 {
			return fmt.Errorf("no .proto files found in api/")
		}

		for _, protoPath := range protoFiles {
			if _, err := os.Stat(protoPath); os.IsNotExist(err) {
				return fmt.Errorf("proto file not found: %s", protoPath)
			}

			moduleName := strings.TrimSuffix(filepath.Base(protoPath), ".proto")
			fmt.Printf("🔄 Generating module %q from %s...\n", moduleName, filepath.Base(protoPath))

			content, err := os.ReadFile(protoPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", protoPath, err)
			}

			pf, err := proto.ParseProto(string(content))
			if err != nil {
				return fmt.Errorf("parsing %s: %w", protoPath, err)
			}

			if err := proto.GenerateModule(pf, moduleName, modulePath, cwd); err != nil {
				return fmt.Errorf("generating module %q: %w", moduleName, err)
			}

			fmt.Printf("✅ Module %q generated successfully!\n\n", moduleName)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
