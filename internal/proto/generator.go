package proto

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// GeneratorData is passed into every code template.
type GeneratorData struct {
	ModuleName       string    // e.g. "example"
	ModulePath       string    // Go module path, e.g. "github.com/user/my-service"
	ServiceName      string    // proto service name, e.g. "ExampleService"
	PackageName      string    // Go package name (lowercase, no hyphens)
	DirPrefix        string    // "" for external, "internal/" for internal modules
	ModuleImportPath string    // full import path for this module's interfaces, e.g. "github.com/user/my-service/example" or "github.com/user/my-service/internal/example"
	RPCs             []RPCInfo // RPCs that belong to this data set
	Messages         []MsgInfo // all proto messages as Go struct info
}

// RPCInfo carries all derived information about one RPC.
type RPCInfo struct {
	Name         string // "CreateExample"
	RequestType  string // "CreateExampleRequest"
	ResponseType string // "CreateExampleResponse"
	HTTPMethod   string // "POST"
	HTTPPath     string // "/examples"
	HasPathID    bool   // true → path ends with /:id
	ReceiverName string // lowercase first word, e.g. "h"
}

// MsgInfo carries information about one proto message mapped to Go.
type MsgInfo struct {
	Name      string
	Fields    []FieldInfo
	IsRequest bool // message name ends with "Request"
}

// FieldInfo carries information about one message field.
type FieldInfo struct {
	ProtoName   string // "user_id"
	GoName      string // "UserID"
	GoType      string // "string", "int32", "[]*SomeMsg", …
	JSONTag     string // `json:"user_id"`
	ValidateTag string // `validate:"required"` or ""
	IsID        bool   // true if field is named "id"
}

// GenerateModule generates all Go files for a module inside projectDir.
// RPCs without the Internal keyword go into <module>/ (with HTTP handler).
// RPCs marked Internal go into internal/<module>/ (no HTTP handler).
func GenerateModule(pf *ProtoFile, moduleName, modulePath, projectDir string) error {
	if len(pf.Services) == 0 {
		return fmt.Errorf("no service defined in proto file")
	}

	svc := pf.Services[0]
	external, internal := buildGeneratorDataPair(pf, svc, moduleName, modulePath)

	funcMap := template.FuncMap{
		"lower":      strings.ToLower,
		"lowerFirst": lowerFirst,
	}

	// Always generate models (shared by both external and internal).
	// Use external data if available, otherwise internal.
	modelData := external
	if len(external.RPCs) == 0 {
		modelData = internal
	}
	if err := writeFile(projectDir, filepath.Join("models", moduleName+"_dto.go"), tmplModels, modelData, funcMap); err != nil {
		return err
	}

	// External module files
	if len(external.RPCs) > 0 {
		extFiles := map[string]string{
			filepath.Join(moduleName, "interface.go"):                                     tmplInterface,
			filepath.Join(moduleName, "handler", "http", moduleName+"_handler.go"):        tmplHandler,
			filepath.Join(moduleName, "service", moduleName+"_service.go"):                tmplService,
			filepath.Join(moduleName, "repository", "mysql", moduleName+"_repository.go"): tmplRepository,
			filepath.Join(moduleName, "_mock", moduleName+"_repository_mock.go"):          tmplMockRepo,
			filepath.Join(moduleName, "_mock", moduleName+"_service_mock.go"):             tmplMockService,
		}
		for relPath, tmplStr := range extFiles {
			if err := writeFile(projectDir, relPath, tmplStr, external, funcMap); err != nil {
				return err
			}
		}
	}

	// Internal module files (no HTTP handler)
	if len(internal.RPCs) > 0 {
		intBase := filepath.Join("internal", moduleName)
		intFiles := map[string]string{
			filepath.Join(intBase, "interface.go"):                                     tmplInterface,
			filepath.Join(intBase, "service", moduleName+"_service.go"):                tmplService,
			filepath.Join(intBase, "repository", "mysql", moduleName+"_repository.go"): tmplRepository,
			filepath.Join(intBase, "_mock", moduleName+"_repository_mock.go"):          tmplMockRepo,
			filepath.Join(intBase, "_mock", moduleName+"_service_mock.go"):             tmplMockService,
		}
		for relPath, tmplStr := range intFiles {
			if err := writeFile(projectDir, relPath, tmplStr, internal, funcMap); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeFile renders a template and writes the result to projectDir/relPath.
func writeFile(projectDir, relPath, tmplStr string, data GeneratorData, funcMap template.FuncMap) error {
	fullPath := filepath.Join(projectDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", relPath, err)
	}
	tmpl, err := template.New(relPath).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parsing template for %s: %w", relPath, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template for %s: %w", relPath, err)
	}
	if err := os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", relPath, err)
	}
	fmt.Printf("  ✔ %s\n", relPath)
	return nil
}

// ReadModuleName reads the Go module path from go.mod in projectDir.
func ReadModuleName(projectDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module declaration not found in go.mod")
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildGeneratorDataPair returns two GeneratorData values:
// one for external RPCs (no Internal keyword) and one for Internal RPCs.
func buildGeneratorDataPair(pf *ProtoFile, svc ServiceDef, moduleName, modulePath string) (external, internal GeneratorData) {
	msgs := buildMessages(pf)

	var extRPCs, intRPCs []RPCInfo
	for _, r := range svc.RPCs {
		var method, path string
		var hasID bool

		if r.HTTPMethod != "" {
			// Explicit route annotation wins over name inference.
			method = r.HTTPMethod
			path = r.HTTPPath
			hasID = strings.Contains(path, "/:id") || strings.Contains(path, "/{id}")
		} else {
			method, path, hasID = inferRoute(r.Name, moduleName)
		}

		info := RPCInfo{
			Name:         r.Name,
			RequestType:  r.RequestType,
			ResponseType: r.ResponseType,
			HTTPMethod:   method,
			HTTPPath:     path,
			HasPathID:    hasID,
		}
		if r.Internal {
			intRPCs = append(intRPCs, info)
		} else {
			extRPCs = append(extRPCs, info)
		}
	}

	base := GeneratorData{
		ModuleName:  moduleName,
		ModulePath:  modulePath,
		ServiceName: svc.Name,
		PackageName: toPackageName(moduleName),
		Messages:    msgs,
	}

	external = base
	external.RPCs = extRPCs
	external.DirPrefix = ""
	external.ModuleImportPath = modulePath + "/" + toPackageName(moduleName)

	internal = base
	internal.RPCs = intRPCs
	internal.DirPrefix = "internal/"
	internal.ModuleImportPath = modulePath + "/internal/" + toPackageName(moduleName)

	return external, internal
}

func buildMessages(pf *ProtoFile) []MsgInfo {
	msgs := make([]MsgInfo, 0, len(pf.Messages))
	for _, m := range pf.Messages {
		isReq := strings.HasSuffix(m.Name, "Request")
		fields := make([]FieldInfo, 0, len(m.Fields))
		for _, f := range m.Fields {
			fi := FieldInfo{
				ProtoName:   f.Name,
				GoName:      toPascalCase(f.Name),
				GoType:      protoTypeToGo(f.Type, f.Repeated),
				JSONTag:     f.Name,
				IsID:        f.Name == "id",
			}
			if isReq {
				fi.ValidateTag = "required"
			}
			fields = append(fields, fi)
		}
		msgs = append(msgs, MsgInfo{
			Name:      m.Name,
			Fields:    fields,
			IsRequest: isReq,
		})
	}
	return msgs
}

// inferRoute derives HTTP method, path, and whether there's a /:id segment.
func inferRoute(rpcName, moduleName string) (method, path string, hasID bool) {
	lower := strings.ToLower(rpcName)
	plural := strings.ToLower(moduleName) + "s"

	switch {
	case strings.HasPrefix(lower, "create"):
		return "POST", "/" + plural, false
	case strings.HasPrefix(lower, "list"):
		return "GET", "/" + plural, false
	case strings.HasPrefix(lower, "get"):
		return "GET", "/" + plural + "/:id", true
	case strings.HasPrefix(lower, "update"):
		return "PUT", "/" + plural + "/:id", true
	case strings.HasPrefix(lower, "delete"):
		return "DELETE", "/" + plural + "/:id", true
	default:
		return "POST", "/" + strings.ToLower(rpcName), false
	}
}

// toPascalCase converts snake_case → PascalCase. Also handles "id" → "ID".
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		upper := strings.ToUpper(p)
		// common Go acronyms
		switch upper {
		case "ID", "URL", "URI", "HTTP", "JSON", "API", "SQL", "UUID":
			b.WriteString(upper)
		default:
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

func toPackageName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// protoTypeToGo maps a proto field type to a Go type string.
func protoTypeToGo(protoType string, repeated bool) string {
	var goType string
	switch protoType {
	case "string":
		goType = "string"
	case "int32":
		goType = "int32"
	case "int64":
		goType = "int64"
	case "uint32":
		goType = "uint32"
	case "uint64":
		goType = "uint64"
	case "float":
		goType = "float32"
	case "double":
		goType = "float64"
	case "bool":
		goType = "bool"
	case "bytes":
		goType = "[]byte"
	default:
		// treat as a message reference
		goType = "*" + protoType
	}
	if repeated {
		if strings.HasPrefix(goType, "*") {
			return "[]*" + goType[1:]
		}
		return "[]" + goType
	}
	return goType
}

// ─────────────────────────────────────────────────────────────────────────────
// Code templates
// ─────────────────────────────────────────────────────────────────────────────

var tmplModels = `// Code generated by a-kit. DO NOT EDIT.
package models
{{range .Messages}}
type {{.Name}} struct {
{{- range .Fields}}
	{{.GoName}} {{.GoType}} ` + "`" + `json:"{{.JSONTag}}"{{if .ValidateTag}} validate:"{{.ValidateTag}}"{{end}}` + "`" + `
{{- end}}
}
{{end}}`

var tmplInterface = `package {{.PackageName}}

import (
	"context"

	"{{.ModulePath}}/models"
)

// {{.ServiceName}}RepositoryInterface defines data-access operations.
type {{.ServiceName}}RepositoryInterface interface {
{{- range .RPCs}}
	{{.Name}}(ctx context.Context, req *models.{{.RequestType}}) (*models.{{.ResponseType}}, error)
{{- end}}
}

// {{.ServiceName}}Interface defines business-logic operations.
type {{.ServiceName}}Interface interface {
{{- range .RPCs}}
	{{.Name}}(ctx context.Context, req *models.{{.RequestType}}) (*models.{{.ResponseType}}, error)
{{- end}}
}
`

var tmplHandler = `package http

import (
	"net/http"

	{{.PackageName}} "{{.ModuleImportPath}}"
	"{{.ModulePath}}/global"
	"{{.ModulePath}}/middlewares"
	"{{.ModulePath}}/models"
	"{{.ModulePath}}/utils/validator"

	"github.com/labstack/echo/v4"
)

type {{.ServiceName}}Handler struct {
	svc {{.PackageName}}.{{.ServiceName}}Interface
	mw  middlewares.GoMiddlewareInterface
}

// New{{.ServiceName}}Handler registers all routes for the {{.PackageName}} module.
func New{{.ServiceName}}Handler(e *echo.Echo, svc {{.PackageName}}.{{.ServiceName}}Interface, mw middlewares.GoMiddlewareInterface) {
	h := &{{.ServiceName}}Handler{svc: svc, mw: mw}
	v1 := e.Group("/v1")
{{- range .RPCs}}
	v1.{{.HTTPMethod}}("{{.HTTPPath}}", h.{{.Name}})
{{- end}}
}
{{range .RPCs}}
func (h *{{$.ServiceName}}Handler) {{.Name}}(c echo.Context) error {
{{- if .HasPathID}}
	var req models.{{.RequestType}}
	req.ID = c.Param("id")
{{- else if eq .HTTPMethod "GET"}}
	var req models.{{.RequestType}}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, global.BadResponse{Code: http.StatusBadRequest, Message: "invalid query params"})
	}
{{- else}}
	var req models.{{.RequestType}}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, global.BadResponse{Code: http.StatusBadRequest, Message: "invalid request body"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, global.BadResponse{Code: http.StatusBadRequest, Message: err.Error()})
	}
{{- end}}
	resp, err := h.svc.{{.Name}}(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, global.BadResponse{Code: http.StatusInternalServerError, Message: err.Error()})
	}
{{- if eq .HTTPMethod "POST"}}
	return c.JSON(http.StatusCreated, global.SuccessResponse{Code: http.StatusCreated, Message: "created", Data: resp})
{{- else if eq .HTTPMethod "DELETE"}}
	return c.JSON(http.StatusOK, global.SuccessResponse{Code: http.StatusOK, Message: "deleted", Data: resp})
{{- else}}
	return c.JSON(http.StatusOK, global.SuccessResponse{Code: http.StatusOK, Message: "ok", Data: resp})
{{- end}}
}
{{end}}`

var tmplService = `package service

import (
	"context"
	"fmt"

	{{.PackageName}} "{{.ModuleImportPath}}"
	"{{.ModulePath}}/models"
)

type {{lowerFirst .ServiceName}} struct {
	repo {{.PackageName}}.{{.ServiceName}}RepositoryInterface
}

// New{{.ServiceName}} creates a new {{.PackageName}} service.
func New{{.ServiceName}}(repo {{.PackageName}}.{{.ServiceName}}RepositoryInterface) {{.PackageName}}.{{.ServiceName}}Interface {
	return &{{lowerFirst .ServiceName}}{repo: repo}
}
{{range .RPCs}}
func (s *{{lowerFirst $.ServiceName}}) {{.Name}}(ctx context.Context, req *models.{{.RequestType}}) (*models.{{.ResponseType}}, error) {
	resp, err := s.repo.{{.Name}}(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("{{.Name}}: %w", err)
	}
	return resp, nil
}
{{end}}`

var tmplRepository = `package mysql

import (
	"context"
	"fmt"

	{{.PackageName}} "{{.ModuleImportPath}}"
	"{{.ModulePath}}/models"

	"gorm.io/gorm"
)

type {{lowerFirst .ServiceName}}Repository struct {
	db *gorm.DB
}

// New{{.ServiceName}}MySQLRepository creates a new {{.PackageName}} MySQL repository.
func New{{.ServiceName}}MySQLRepository(db *gorm.DB) {{.PackageName}}.{{.ServiceName}}RepositoryInterface {
	return &{{lowerFirst .ServiceName}}Repository{db: db}
}
{{range .RPCs}}
func (r *{{lowerFirst $.ServiceName}}Repository) {{.Name}}(ctx context.Context, req *models.{{.RequestType}}) (*models.{{.ResponseType}}, error) {
	// TODO: implement database logic
	return nil, fmt.Errorf("{{.Name}}: not implemented")
}
{{end}}`

var tmplMockRepo = `package mock

import (
	"context"

	"{{.ModulePath}}/models"

	"github.com/stretchr/testify/mock"
)

// Mock{{.ServiceName}}Repository is a testify mock for {{.ServiceName}}RepositoryInterface.
type Mock{{.ServiceName}}Repository struct {
	mock.Mock
}
{{range .RPCs}}
func (m *Mock{{$.ServiceName}}Repository) {{.Name}}(ctx context.Context, req *models.{{.RequestType}}) (*models.{{.ResponseType}}, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.{{.ResponseType}}), args.Error(1)
}
{{end}}`

var tmplMockService = `package mock

import (
	"context"

	"{{.ModulePath}}/models"

	"github.com/stretchr/testify/mock"
)

// Mock{{.ServiceName}} is a testify mock for {{.ServiceName}}Interface.
type Mock{{.ServiceName}} struct {
	mock.Mock
}
}
{{range .RPCs}}
func (m *Mock{{$.ServiceName}}) {{.Name}}(ctx context.Context, req *models.{{.RequestType}}) (*models.{{.ResponseType}}, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.{{.ResponseType}}), args.Error(1)
}
{{end}}`
