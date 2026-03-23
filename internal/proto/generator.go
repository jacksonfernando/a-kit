package proto

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	ModuleImportPath string    // full import path for this module's interfaces
	RPCs             []RPCInfo // RPCs that belong to this data set
	Messages         []MsgInfo // all proto messages as Go struct info
	NeedsEmpty       bool      // true when any RPC response is Empty (google.protobuf.Empty)
}

// RPCInfo carries all derived information about one RPC.
type RPCInfo struct {
	Name         string      // "CreateExample"
	RequestType  string      // "CreateExampleRequest"
	ResponseType string      // "CreateExampleResponse" or "Example"
	HTTPMethod   string      // "POST"
	HTTPPath     string      // "/v1/examples"  (full path, Echo-compatible)
	HasPathID    bool        // true → at least one path parameter exists
	PathParams   []PathParam // named path parameters for handler binding
	NeedsBody    bool        // true → method sends a body (POST/PUT/PATCH)
	ReceiverName string      // lowercase first word, e.g. "h"
}

// PathParam describes a single path parameter extracted from the route.
type PathParam struct {
	EchoParam string // param name as used in Echo: c.Param("name")
	GoField   string // PascalCase Go field name: "Name"
	Nested    bool   // true if it refers to a nested field (e.g. "order.name")
	FieldPath string // original field path for reference: "order.name"
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
	QueryTag    string // `query:"user_id"` — used by Echo to bind query string params
	ValidateTag string // `validate:"required"` or `validate:"required,dive"` or ""
	IsID        bool   // true if field is named "id"
	Repeated    bool   // true if the field is a slice (repeated in proto)
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
	needsEmpty := false

	var extRPCs, intRPCs []RPCInfo
	for _, r := range svc.RPCs {
		if r.ResponseType == "Empty" {
			needsEmpty = true
		}

		var method, echoPath string
		var pathParams []PathParam

		if r.HTTPMethod != "" {
			method = r.HTTPMethod
			// Detect if path is Google API format (contains {…}) or plain Echo format
			if strings.Contains(r.HTTPPath, "{") {
				echoPath, pathParams = googlePathToEcho(r.HTTPPath)
			} else {
				echoPath = r.HTTPPath
				// Extract :param style params from plain Echo paths
				for _, seg := range strings.Split(echoPath, "/") {
					if strings.HasPrefix(seg, ":") {
						p := strings.TrimPrefix(seg, ":")
						pathParams = append(pathParams, pathParamFromName(p))
					}
				}
			}
		} else {
			method, echoPath, pathParams = inferRoute(r.Name, moduleName)
		}

		info := RPCInfo{
			Name:         r.Name,
			RequestType:  r.RequestType,
			ResponseType: r.ResponseType,
			HTTPMethod:   method,
			HTTPPath:     echoPath,
			HasPathID:    len(pathParams) > 0,
			PathParams:   pathParams,
			NeedsBody:    method == "POST" || method == "PUT" || method == "PATCH",
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
		NeedsEmpty:  needsEmpty,
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
				ProtoName: f.Name,
				GoName:    toPascalCase(f.Name),
				GoType:    protoTypeToGo(f.Type, f.Repeated),
				JSONTag:   f.Name,
				QueryTag:  f.Name,
				IsID:      f.Name == "id",
				Repeated:  f.Repeated,
			}
			if isReq {
				if f.Repeated && !isPrimitiveType(f.Type) {
					// repeated message type: dive so each element is validated recursively
					fi.ValidateTag = "required,dive"
				} else {
					fi.ValidateTag = "required"
				}
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

// inferRoute derives HTTP method, path (full, Echo-compatible), and path params from the RPC name.
func inferRoute(rpcName, moduleName string) (method, path string, pathParams []PathParam) {
	lower := strings.ToLower(rpcName)
	plural := "/v1/" + strings.ToLower(moduleName) + "s"

	switch {
	case strings.HasPrefix(lower, "create"):
		return "POST", plural, nil
	case strings.HasPrefix(lower, "list"):
		return "GET", plural, nil
	case strings.HasPrefix(lower, "get"):
		return "GET", plural + "/:id", []PathParam{pathParamFromName("id")}
	case strings.HasPrefix(lower, "update"):
		return "PATCH", plural + "/:id", []PathParam{pathParamFromName("id")}
	case strings.HasPrefix(lower, "delete"):
		return "DELETE", plural + "/:id", []PathParam{pathParamFromName("id")}
	default:
		return "POST", "/v1/" + strings.ToLower(rpcName), nil
	}
}

// googlePathToEcho converts a Google API HTTP path template to an Echo-compatible path.
// e.g. "/v1/{name=examples/*}" → "/v1/examples/:name"
// e.g. "/v1/{name=examples/*}:search" → "/v1/examples/:name/search"
func googlePathToEcho(template string) (echoPath string, pathParams []PathParam) {
	// Detect and strip custom method suffix (:verb at the end, outside {})
	customMethod := ""
	braceDepth := 0
	lastColon := -1
	for i, ch := range template {
		switch ch {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case ':':
			if braceDepth == 0 {
				lastColon = i
			}
		}
	}
	if lastColon > 0 {
		customMethod = template[lastColon+1:]
		template = template[:lastColon]
	}

	// Replace {fieldPath=collection/*} and {fieldPath} with collection/:param
	reParam := regexp.MustCompile(`\{([\w.]+)(?:=([^}]*))?\}`)
	echoPath = reParam.ReplaceAllStringFunc(template, func(match string) string {
		groups := reParam.FindStringSubmatch(match)
		fieldPath := groups[1] // e.g. "name" or "order.name"
		pattern := groups[2]   // e.g. "examples/*" or ""

		// Use the last segment of a dotted field path as the Echo param name.
		parts := strings.Split(fieldPath, ".")
		paramName := parts[len(parts)-1]
		nested := len(parts) > 1

		pp := PathParam{
			EchoParam: paramName,
			GoField:   toPascalCase(paramName),
			Nested:    nested,
			FieldPath: fieldPath,
		}
		pathParams = append(pathParams, pp)

		if pattern != "" {
			// Extract the innermost collection name from the pattern.
			// "examples/*" → "examples", "projects/*/orders/*" → "orders"
			segs := strings.Split(strings.TrimSuffix(pattern, "/*"), "/")
			collection := segs[len(segs)-1]
			if collection == "*" && len(segs) >= 2 {
				collection = segs[len(segs)-2]
			}
			return collection + "/:" + paramName
		}
		return ":" + paramName
	})

	if customMethod != "" {
		echoPath += "/" + customMethod
	}
	return echoPath, pathParams
}

// pathParamFromName builds a PathParam for a simple (non-nested) parameter name.
func pathParamFromName(name string) PathParam {
	return PathParam{
		EchoParam: name,
		GoField:   toPascalCase(name),
		Nested:    false,
		FieldPath: name,
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
		// common Go acronyms (singular and plural)
		switch upper {
		case "ID", "URL", "URI", "HTTP", "JSON", "API", "SQL", "UUID":
			b.WriteString(upper)
		case "IDS":
			b.WriteString("IDs")
		case "URLS", "URIS":
			b.WriteString(strings.ToUpper(p[:len(p)-1]) + "s")
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

func isPrimitiveType(protoType string) bool {
	switch protoType {
	case "string", "int32", "int64", "uint32", "uint64",
		"float", "double", "bool", "bytes",
		"google.protobuf.FieldMask":
		return true
	}
	return false
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
	case "google.protobuf.FieldMask":
		goType = "[]string" // field paths, e.g. ["display_name", "description"]
	case "google.protobuf.Empty", "Empty":
		goType = "struct{}"
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
{{if .NeedsEmpty}}
// Empty represents an absent response body (mapped from google.protobuf.Empty).
type Empty struct{}
{{end}}
{{range .Messages}}
type {{.Name}} struct {
{{- range .Fields}}
	{{.GoName}} {{.GoType}} ` + "`" + `json:"{{.JSONTag}}" query:"{{.QueryTag}}"{{if .ValidateTag}} validate:"{{.ValidateTag}}"{{end}}` + "`" + `
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

// New{{.ServiceName}}Handler registers all HTTP routes for the {{.PackageName}} module.
func New{{.ServiceName}}Handler(e *echo.Echo, svc {{.PackageName}}.{{.ServiceName}}Interface, mw middlewares.GoMiddlewareInterface) {
	h := &{{.ServiceName}}Handler{svc: svc, mw: mw}
{{- range .RPCs}}
	e.{{.HTTPMethod}}("{{.HTTPPath}}", h.{{.Name}})
{{- end}}
}
{{range .RPCs}}
func (h *{{$.ServiceName}}Handler) {{.Name}}(c echo.Context) error {
	var req models.{{.RequestType}}

	// Bind query params (GET) or request body (POST/PUT/PATCH).
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, global.BadResponse{Code: http.StatusBadRequest, Message: "invalid request"})
	}
{{- range .PathParams}}
{{- if .Nested}}
	// TODO: bind path param ":{{.EchoParam}}" to nested field {{.FieldPath}}
{{- else}}
	req.{{.GoField}} = c.Param("{{.EchoParam}}")
{{- end}}
{{- end}}
{{- if .NeedsBody}}
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
