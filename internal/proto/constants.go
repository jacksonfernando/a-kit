package proto

import "net/http"

// frameKind identifies the type of parser nesting context on the stack.
type frameKind string

const (
	frameKindService    frameKind = "service"
	frameKindMessage    frameKind = "message"
	frameKindRPCOptions frameKind = "rpc_options"
	frameKindHTTPOption frameKind = "http_option"
)

// protoModifier represents a proto field modifier keyword.
type protoModifier string

const (
	modifierRepeated protoModifier = "repeated"
	modifierOptional protoModifier = "optional"
)

// HTTPMethod represents an HTTP verb.
type HTTPMethod string

const (
	HTTPMethodGET    HTTPMethod = http.MethodGet
	HTTPMethodPOST   HTTPMethod = http.MethodPost
	HTTPMethodPUT    HTTPMethod = http.MethodPut
	HTTPMethodPATCH  HTTPMethod = http.MethodPatch
	HTTPMethodDELETE HTTPMethod = http.MethodDelete
)

const protoTypeEmpty = "google.protobuf.Empty"

// templateName is the filename of an embedded Go template.
type templateName string

const (
	tmplModels         templateName = "models.tmpl"
	tmplInterface      templateName = "interface.tmpl"
	tmplHandler        templateName = "handler.tmpl"
	tmplHandlerTest    templateName = "handler_test.tmpl"
	tmplService        templateName = "service.tmpl"
	tmplServiceTest    templateName = "service_test.tmpl"
	tmplRepository     templateName = "repository.tmpl"
	tmplMockRepository templateName = "mock_repository.tmpl"
	tmplMockService    templateName = "mock_service.tmpl"
)

// dirSegment is a path component used when building output directory trees.
type dirSegment string

const (
	dirModels     dirSegment = "models"
	dirHandler    dirSegment = "handler"
	dirHTTP       dirSegment = "http"
	dirService    dirSegment = "service"
	dirRepository dirSegment = "repository"
	dirMySQL      dirSegment = "mysql"
	dirMock       dirSegment = "_mock"
	dirInternal   dirSegment = "internal"
)

const (
	suffixRequest     = "Request"
	fieldIDName       = "id"
	goModPrefix       = "module "
	dirPrefixInternal = "internal/"
)
