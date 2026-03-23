package proto

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

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
	HasBodyRPCs      bool      // true if any RPC in this set needs a request body
}

// RPCInfo carries all derived information about one RPC.
type RPCInfo struct {
	Name         string     // "CreateExample"
	RequestType  string     // "CreateExampleRequest"
	ResponseType string     // "CreateExampleResponse" or "Example"
	HTTPMethod   HTTPMethod // "POST"
	HTTPPath     string     // "/v1/examples"  (full path, Echo-compatible)
	HasPathID    bool       // true → at least one path parameter exists
	PathParams   []PathParam // named path parameters for handler binding
	NeedsBody    bool       // true → method sends a body (POST/PUT/PATCH)
	ReceiverName string     // lowercase first word, e.g. "h"

	// Test data — populated during code generation
	TestURL         string   // URL with :params replaced, e.g. "/v1/examples/test-name"
	TestParamNames  []string // Echo param names for c.SetParamNames(...)
	TestParamValues []string // Echo param values for c.SetParamValues(...)
	TestQueryString string   // Query string for GET, e.g. "?page_size=10&filter=test"
	TestBody        string   // JSON body for POST/PUT/PATCH
	ExpectedStatus  int      // Expected HTTP status code (200 or 201)
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

//go:embed templates
var protoTmplFS embed.FS

func mustProtoTmpl(name templateName) string {
	b, err := protoTmplFS.ReadFile("templates/" + string(name))
	if err != nil {
		panic("a-kit: missing template " + string(name) + ": " + err.Error())
	}
	return string(b)
}

// GenerateModule generates all Go files for a module inside projectDir.
func GenerateModule(pf *ProtoFile, moduleName, modulePath, projectDir string) error {
	if len(pf.Services) == 0 {
		return fmt.Errorf("no service defined in proto file")
	}

	svc := pf.Services[0]
	external, internal := buildGeneratorDataPair(pf, svc, moduleName, modulePath)

	funcMap := template.FuncMap{
		"lower":      strings.ToLower,
		"lowerFirst": lowerFirst,
		"methodTitle": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		},
	}

	// Always generate models (shared by both external and internal).
	// Use external data if available, otherwise internal.
	modelData := external
	if len(external.RPCs) == 0 {
		modelData = internal
	}
	if err := writeFile(projectDir, filepath.Join(string(dirModels), moduleName+"_dto.go"), mustProtoTmpl(tmplModels), modelData, funcMap); err != nil {
		return err
	}

	// External module files
	if len(external.RPCs) > 0 {
		extFiles := map[string]string{
			filepath.Join(moduleName, "interface.go"): mustProtoTmpl(tmplInterface),
			filepath.Join(moduleName, string(dirHandler), string(dirHTTP), moduleName+"_handler.go"):      mustProtoTmpl(tmplHandler),
			filepath.Join(moduleName, string(dirService), moduleName+"_service.go"):                       mustProtoTmpl(tmplService),
			filepath.Join(moduleName, string(dirRepository), string(dirMySQL), moduleName+"_repository.go"): mustProtoTmpl(tmplRepository),
			filepath.Join(moduleName, string(dirMock), moduleName+"_repository_mock.go"):                  mustProtoTmpl(tmplMockRepository),
			filepath.Join(moduleName, string(dirMock), moduleName+"_service_mock.go"):                     mustProtoTmpl(tmplMockService),
			filepath.Join(moduleName, string(dirHandler), string(dirHTTP), moduleName+"_handler_test.go"): mustProtoTmpl(tmplHandlerTest),
			filepath.Join(moduleName, string(dirService), moduleName+"_service_test.go"):                  mustProtoTmpl(tmplServiceTest),
		}
		for relPath, tmplStr := range extFiles {
			if err := writeFile(projectDir, relPath, tmplStr, external, funcMap); err != nil {
				return err
			}
		}
	}

	// Internal module files (no HTTP handler)
	if len(internal.RPCs) > 0 {
		intBase := filepath.Join(string(dirInternal), moduleName)
		intFiles := map[string]string{
			filepath.Join(intBase, "interface.go"): mustProtoTmpl(tmplInterface),
			filepath.Join(intBase, string(dirService), moduleName+"_service.go"):                       mustProtoTmpl(tmplService),
			filepath.Join(intBase, string(dirRepository), string(dirMySQL), moduleName+"_repository.go"): mustProtoTmpl(tmplRepository),
			filepath.Join(intBase, string(dirMock), moduleName+"_repository_mock.go"):                  mustProtoTmpl(tmplMockRepository),
			filepath.Join(intBase, string(dirMock), moduleName+"_service_mock.go"):                     mustProtoTmpl(tmplMockService),
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
		if strings.HasPrefix(line, goModPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, goModPrefix)), nil
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

		var method HTTPMethod
		var echoPath string
		var pathParams []PathParam

		method = r.HTTPMethod
		echoPath = r.HTTPPath

		switch {
		case method == "":
			method, echoPath, pathParams = inferRoute(r.Name, moduleName)
		case strings.Contains(r.HTTPPath, "{"):
			// Google API style path template: {name=examples/*} → :name
			echoPath, pathParams = googlePathToEcho(r.HTTPPath)
		default:
			// Plain Echo path: extract :param segments
			for _, seg := range strings.Split(echoPath, "/") {
				if strings.HasPrefix(seg, ":") {
					p := strings.TrimPrefix(seg, ":")
					pathParams = append(pathParams, pathParamFromName(p))
				}
			}
		}

		info := RPCInfo{
			Name:         r.Name,
			RequestType:  r.RequestType,
			ResponseType: r.ResponseType,
			HTTPMethod:   method,
			HTTPPath:     echoPath,
			HasPathID:    len(pathParams) > 0,
			PathParams:   pathParams,
			NeedsBody:    method == HTTPMethodPOST || method == HTTPMethodPUT || method == HTTPMethodPATCH,
		}

		// populate test data
		testURL, testParamNames, testParamValues := computeTestURL(echoPath, pathParams)
		info.TestURL = testURL
		info.TestParamNames = testParamNames
		info.TestParamValues = testParamValues
		info.ExpectedStatus = expectedHTTPStatus(method)
		info.TestQueryString = computeTestQueryString(r.RequestType, pathParams, msgs)
		if info.NeedsBody {
			info.TestBody = computeTestBody(r.RequestType, msgs)
			info.TestQueryString = ""
		}

		target := &extRPCs
		if r.Internal {
			target = &intRPCs
		}
		*target = append(*target, info)
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
	internal.DirPrefix = dirPrefixInternal
	internal.ModuleImportPath = modulePath + "/" + dirPrefixInternal + toPackageName(moduleName)

	for _, r := range external.RPCs {
		if r.NeedsBody {
			external.HasBodyRPCs = true
			break
		}
	}
	for _, r := range internal.RPCs {
		if r.NeedsBody {
			internal.HasBodyRPCs = true
			break
		}
	}

	return external, internal
}

func buildMessages(pf *ProtoFile) []MsgInfo {
	msgs := make([]MsgInfo, 0, len(pf.Messages))
	for _, m := range pf.Messages {
		isReq := strings.HasSuffix(m.Name, suffixRequest)
		fields := make([]FieldInfo, 0, len(m.Fields))
		for _, f := range m.Fields {
			fi := FieldInfo{
				ProtoName: f.Name,
				GoName:    toPascalCase(f.Name),
				GoType:    protoTypeToGo(f.Type, f.Repeated),
				JSONTag:   f.Name,
				QueryTag:  f.Name,
				IsID:      f.Name == fieldIDName,
				Repeated:  f.Repeated,
			}
			if isReq {
				fi.ValidateTag = "required"
				if f.Repeated && !isPrimitiveType(f.Type) {
					// repeated message type: dive so each element is validated recursively
					fi.ValidateTag = "required,dive"
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
func inferRoute(rpcName, moduleName string) (method HTTPMethod, path string, pathParams []PathParam) {
	lower := strings.ToLower(rpcName)
	plural := "/v1/" + strings.ToLower(moduleName) + "s"

	switch {
	case strings.HasPrefix(lower, "create"):
		return HTTPMethodPOST, plural, nil
	case strings.HasPrefix(lower, "list"):
		return HTTPMethodGET, plural, nil
	case strings.HasPrefix(lower, "get"):
		return HTTPMethodGET, plural + "/:id", []PathParam{pathParamFromName("id")}
	case strings.HasPrefix(lower, "update"):
		return HTTPMethodPATCH, plural + "/:id", []PathParam{pathParamFromName("id")}
	case strings.HasPrefix(lower, "delete"):
		return HTTPMethodDELETE, plural + "/:id", []PathParam{pathParamFromName("id")}
	default:
		return HTTPMethodPOST, "/v1/" + strings.ToLower(rpcName), nil
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

// buildMsgMap creates a name→MsgInfo lookup.
func buildMsgMap(msgs []MsgInfo) map[string]MsgInfo {
	m := make(map[string]MsgInfo, len(msgs))
	for _, msg := range msgs {
		m[msg.Name] = msg
	}
	return m
}

// expectedHTTPStatus returns 201 for POST, 200 for everything else.
func expectedHTTPStatus(method HTTPMethod) int {
	if method == HTTPMethodPOST {
		return 201
	}
	return 200
}

// computeTestURL replaces `:param` segments in an Echo path with "test-<param>" values.
func computeTestURL(echoPath string, pathParams []PathParam) (testURL string, names, values []string) {
	testURL = echoPath
	for _, pp := range pathParams {
		testVal := "test-" + pp.EchoParam
		testURL = strings.ReplaceAll(testURL, ":"+pp.EchoParam, testVal)
		names = append(names, pp.EchoParam)
		values = append(values, testVal)
	}
	return testURL, names, values
}

// computeTestQueryString builds a query string from request fields that are NOT path params.
// Only used for GET/DELETE requests (no body).
func computeTestQueryString(requestType string, pathParams []PathParam, msgs []MsgInfo) string {
	var msg *MsgInfo
	for i := range msgs {
		if msgs[i].Name == requestType {
			msg = &msgs[i]
			break
		}
	}
	if msg == nil {
		return ""
	}

	skip := map[string]bool{}
	for _, pp := range pathParams {
		skip[pp.GoField] = true
	}

	var pairs []string
	for _, f := range msg.Fields {
		if skip[f.GoName] {
			continue
		}
		pairs = append(pairs, f.JSONTag+"="+sampleQueryValue(f.GoType))
	}
	if len(pairs) == 0 {
		return ""
	}
	return "?" + strings.Join(pairs, "&")
}

// sampleQueryValue returns a URL-safe sample value for the given Go type.
func sampleQueryValue(goType string) string {
	switch goType {
	case "int32", "int64", "uint32", "uint64":
		return "10"
	case "float32", "float64":
		return "1.0"
	case "bool":
		return "true"
	default:
		return "test"
	}
}

// computeTestBody generates a sample JSON object for the given request type.
func computeTestBody(requestType string, msgs []MsgInfo) string {
	msgMap := buildMsgMap(msgs)
	msg, ok := msgMap[requestType]
	if !ok {
		return "{}"
	}
	return sampleJSONObject(msg.Fields, msgMap)
}

func sampleJSONObject(fields []FieldInfo, msgMap map[string]MsgInfo) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		val := sampleJSONValue(f, msgMap)
		parts = append(parts, fmt.Sprintf(`%q: %s`, f.JSONTag, val))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func sampleJSONValue(f FieldInfo, msgMap map[string]MsgInfo) string {
	gt := f.GoType
	switch gt {
	case "string":
		return `"test"`
	case "int32", "int64", "uint32", "uint64":
		return `1`
	case "float32", "float64":
		return `1.0`
	case "bool":
		return `true`
	case "[]byte":
		return `""`
	case "[]string":
		return `["field_name"]`
	case "struct{}":
		return `{}`
	default:
		if strings.HasPrefix(gt, "[]*") {
			elemType := gt[3:]
			if msg, ok := msgMap[elemType]; ok {
				return "[" + sampleJSONObject(msg.Fields, msgMap) + "]"
			}
			return `[{}]`
		}
		if strings.HasPrefix(gt, "[]") {
			return `["test"]`
		}
		if strings.HasPrefix(gt, "*") {
			typeName := gt[1:]
			if msg, ok := msgMap[typeName]; ok {
				return sampleJSONObject(msg.Fields, msgMap)
			}
			return `{}`
		}
		return `"test"`
	}
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
