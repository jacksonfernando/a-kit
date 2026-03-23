package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─── toPascalCase ─────────────────────────────────────────────────────────────

func TestToPascalCase(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"name", "Name"},
		{"display_name", "DisplayName"},
		{"page_size", "PageSize"},
		{"order_by", "OrderBy"},
		{"next_page_token", "NextPageToken"},
		// acronyms
		{"id", "ID"},
		{"ids", "IDs"},
		{"user_id", "UserID"},
		{"order_ids", "OrderIDs"},
		{"url", "URL"},
		{"urls", "URLs"},
		{"uri", "URI"},
		{"uris", "URIs"},
		{"http_method", "HTTPMethod"},
		{"api_key", "APIKey"},
		{"json_body", "JSONBody"},
		{"sql_query", "SQLQuery"},
		{"uuid", "UUID"},
		// edge cases
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, toPascalCase(tc.input))
		})
	}
}

// ─── toPackageName ────────────────────────────────────────────────────────────

func TestToPackageName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"order", "order"},
		{"Order", "order"},
		{"my-service", "myservice"},
		{"my_service", "myservice"},
		{"MyService", "myservice"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, toPackageName(tc.input))
		})
	}
}

// ─── lowerFirst ───────────────────────────────────────────────────────────────

func TestLowerFirst(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"OrderService", "orderService"},
		{"order", "order"},
		{"", ""},
		{"O", "o"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, lowerFirst(tc.input))
		})
	}
}

// ─── protoTypeToGo ────────────────────────────────────────────────────────────

func TestProtoTypeToGo(t *testing.T) {
	cases := []struct {
		protoType string
		repeated  bool
		want      string
	}{
		// scalar types
		{"string", false, "string"},
		{"int32", false, "int32"},
		{"int64", false, "int64"},
		{"uint32", false, "uint32"},
		{"uint64", false, "uint64"},
		{"float", false, "float32"},
		{"double", false, "float64"},
		{"bool", false, "bool"},
		{"bytes", false, "[]byte"},
		// well-known types
		{"google.protobuf.FieldMask", false, "[]string"},
		{"google.protobuf.Empty", false, "struct{}"},
		{"Empty", false, "struct{}"},
		// message reference
		{"Order", false, "*Order"},
		{"CreateOrderResponse", false, "*CreateOrderResponse"},
		// repeated scalars
		{"string", true, "[]string"},
		{"int32", true, "[]int32"},
		{"bool", true, "[]bool"},
		// repeated message
		{"Order", true, "[]*Order"},
		{"Example", true, "[]*Example"},
		// repeated well-known
		{"google.protobuf.FieldMask", true, "[][]string"},
	}

	for _, tc := range cases {
		name := tc.protoType
		if tc.repeated {
			name = "repeated " + name
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, protoTypeToGo(tc.protoType, tc.repeated))
		})
	}
}

// ─── isPrimitiveType ──────────────────────────────────────────────────────────

func TestIsPrimitiveType(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"string", true},
		{"int32", true},
		{"int64", true},
		{"uint32", true},
		{"uint64", true},
		{"float", true},
		{"double", true},
		{"bool", true},
		{"bytes", true},
		{"google.protobuf.FieldMask", true},
		// message types are not primitive
		{"Order", false},
		{"Example", false},
		{"google.protobuf.Empty", false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, isPrimitiveType(tc.input))
		})
	}
}

// ─── inferRoute ───────────────────────────────────────────────────────────────

func TestInferRoute(t *testing.T) {
	cases := []struct {
		rpcName    string
		moduleName string
		method     HTTPMethod
		path       string
		hasParams  bool
	}{
		{"CreateOrder", "order", HTTPMethodPOST, "/v1/orders", false},
		{"ListOrders", "order", HTTPMethodGET, "/v1/orders", false},
		{"GetOrder", "order", HTTPMethodGET, "/v1/orders/:id", true},
		{"UpdateOrder", "order", HTTPMethodPATCH, "/v1/orders/:id", true},
		{"DeleteOrder", "order", HTTPMethodDELETE, "/v1/orders/:id", true},
		{"ProcessRefund", "order", HTTPMethodPOST, "/v1/processrefund", false},
		{"CreateExample", "example", HTTPMethodPOST, "/v1/examples", false},
		{"GetExample", "example", HTTPMethodGET, "/v1/examples/:id", true},
	}

	for _, tc := range cases {
		t.Run(tc.rpcName, func(t *testing.T) {
			method, path, params := inferRoute(tc.rpcName, tc.moduleName)
			assert.Equal(t, tc.method, method)
			assert.Equal(t, tc.path, path)
			if tc.hasParams {
				assert.NotEmpty(t, params)
			} else {
				assert.Empty(t, params)
			}
		})
	}
}

// ─── googlePathToEcho ─────────────────────────────────────────────────────────

func TestGooglePathToEcho(t *testing.T) {
	cases := []struct {
		name          string
		template      string
		wantPath      string
		wantParamName string // first param, empty if none
		wantNested    bool
	}{
		{
			name:          "simple resource name",
			template:      "/v1/{name=orders/*}",
			wantPath:      "/v1/orders/:name",
			wantParamName: "name",
		},
		{
			name:          "nested field path",
			template:      "/v1/{order.name=orders/*}",
			wantPath:      "/v1/orders/:name",
			wantParamName: "name",
			wantNested:    true,
		},
		{
			name:          "custom method suffix",
			template:      "/v1/orders:search",
			wantPath:      "/v1/orders/search",
			wantParamName: "",
		},
		{
			name:          "path param with custom method",
			template:      "/v1/{name=orders/*}:cancel",
			wantPath:      "/v1/orders/:name/cancel",
			wantParamName: "name",
		},
		{
			name:          "bare param no collection",
			template:      "/v1/{id}",
			wantPath:      "/v1/:id",
			wantParamName: "id",
		},
		{
			name:          "no params",
			template:      "/v1/orders",
			wantPath:      "/v1/orders",
			wantParamName: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			echoPath, params := googlePathToEcho(tc.template)
			assert.Equal(t, tc.wantPath, echoPath)
			if tc.wantParamName != "" {
				if assert.NotEmpty(t, params) {
					assert.Equal(t, tc.wantParamName, params[0].EchoParam)
					assert.Equal(t, tc.wantNested, params[0].Nested)
				}
			} else {
				assert.Empty(t, params)
			}
		})
	}
}

// ─── computeTestURL ───────────────────────────────────────────────────────────

func TestComputeTestURL(t *testing.T) {
	cases := []struct {
		name        string
		echoPath    string
		pathParams  []PathParam
		wantURL     string
		wantNames   []string
		wantValues  []string
	}{
		{
			name:       "no params",
			echoPath:   "/v1/orders",
			pathParams: nil,
			wantURL:    "/v1/orders",
		},
		{
			name:     "single path param",
			echoPath: "/v1/orders/:name",
			pathParams: []PathParam{
				{EchoParam: "name", GoField: "Name"},
			},
			wantURL:    "/v1/orders/test-name",
			wantNames:  []string{"name"},
			wantValues: []string{"test-name"},
		},
		{
			name:     "multiple path params",
			echoPath: "/v1/projects/:project/orders/:order",
			pathParams: []PathParam{
				{EchoParam: "project", GoField: "Project"},
				{EchoParam: "order", GoField: "Order"},
			},
			wantURL:    "/v1/projects/test-project/orders/test-order",
			wantNames:  []string{"project", "order"},
			wantValues: []string{"test-project", "test-order"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, names, values := computeTestURL(tc.echoPath, tc.pathParams)
			assert.Equal(t, tc.wantURL, url)
			assert.Equal(t, tc.wantNames, names)
			assert.Equal(t, tc.wantValues, values)
		})
	}
}

// ─── expectedHTTPStatus ───────────────────────────────────────────────────────

func TestExpectedHTTPStatus(t *testing.T) {
	cases := []struct {
		method HTTPMethod
		want   int
	}{
		{HTTPMethodPOST, 201},
		{HTTPMethodGET, 200},
		{HTTPMethodPATCH, 200},
		{HTTPMethodPUT, 200},
		{HTTPMethodDELETE, 200},
	}
	for _, tc := range cases {
		t.Run(string(tc.method), func(t *testing.T) {
			assert.Equal(t, tc.want, expectedHTTPStatus(tc.method))
		})
	}
}

// ─── sampleJSONValue ──────────────────────────────────────────────────────────

func TestSampleJSONValue(t *testing.T) {
	msgMap := map[string]MsgInfo{
		"Order": {
			Name: "Order",
			Fields: []FieldInfo{
				{ProtoName: "name", GoName: "Name", GoType: "string", JSONTag: "name"},
				{ProtoName: "amount", GoName: "Amount", GoType: "float64", JSONTag: "amount"},
			},
		},
	}

	cases := []struct {
		name string
		field FieldInfo
		want string
	}{
		{"string", FieldInfo{GoType: "string"}, `"test"`},
		{"int32", FieldInfo{GoType: "int32"}, `1`},
		{"int64", FieldInfo{GoType: "int64"}, `1`},
		{"float64", FieldInfo{GoType: "float64"}, `1.0`},
		{"bool", FieldInfo{GoType: "bool"}, `true`},
		{"[]byte", FieldInfo{GoType: "[]byte"}, `""`},
		{"[]string", FieldInfo{GoType: "[]string"}, `["field_name"]`},
		{"struct{}", FieldInfo{GoType: "struct{}"}, `{}`},
		{"*Order (known msg)", FieldInfo{GoType: "*Order"}, `{"name": "test", "amount": 1.0}`},
		{"[]*Order (repeated msg)", FieldInfo{GoType: "[]*Order"}, `[{"name": "test", "amount": 1.0}]`},
		{"*Unknown", FieldInfo{GoType: "*Unknown"}, `{}`},
		{"[]Unknown", FieldInfo{GoType: "[]Unknown"}, `["test"]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sampleJSONValue(tc.field, msgMap))
		})
	}
}

// ─── computeTestQueryString ───────────────────────────────────────────────────

func TestComputeTestQueryString(t *testing.T) {
	msgs := []MsgInfo{
		{
			Name:      "ListOrdersRequest",
			IsRequest: true,
			Fields: []FieldInfo{
				{ProtoName: "page_size", GoName: "PageSize", GoType: "int32", JSONTag: "page_size"},
				{ProtoName: "filter", GoName: "Filter", GoType: "string", JSONTag: "filter"},
			},
		},
		{
			Name:      "GetOrderRequest",
			IsRequest: true,
			Fields: []FieldInfo{
				{ProtoName: "name", GoName: "Name", GoType: "string", JSONTag: "name"},
			},
		},
	}

	cases := []struct {
		name        string
		requestType string
		pathParams  []PathParam
		wantContains []string
		wantEmpty   bool
	}{
		{
			name:         "list — all fields become query params",
			requestType:  "ListOrdersRequest",
			pathParams:   nil,
			wantContains: []string{"page_size=10", "filter=test"},
		},
		{
			name:        "get — path param field is skipped",
			requestType: "GetOrderRequest",
			pathParams:  []PathParam{{EchoParam: "name", GoField: "Name"}},
			wantEmpty:   true,
		},
		{
			name:        "unknown request type",
			requestType: "NonExistentRequest",
			wantEmpty:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qs := computeTestQueryString(tc.requestType, tc.pathParams, msgs)
			if tc.wantEmpty {
				assert.Empty(t, qs)
			} else {
				for _, s := range tc.wantContains {
					assert.Contains(t, qs, s)
				}
			}
		})
	}
}

// ─── computeTestBody ──────────────────────────────────────────────────────────

func TestComputeTestBody(t *testing.T) {
	msgs := []MsgInfo{
		{
			Name:      "CreateOrderRequest",
			IsRequest: true,
			Fields: []FieldInfo{
				{ProtoName: "order_id", GoName: "OrderID", GoType: "string", JSONTag: "order_id"},
				{ProtoName: "amount", GoName: "Amount", GoType: "float64", JSONTag: "amount"},
			},
		},
	}

	cases := []struct {
		name         string
		requestType  string
		wantContains []string
		wantFallback bool
	}{
		{
			name:         "known type generates JSON",
			requestType:  "CreateOrderRequest",
			wantContains: []string{`"order_id"`, `"amount"`},
		},
		{
			name:         "unknown type returns empty object",
			requestType:  "UnknownRequest",
			wantFallback: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := computeTestBody(tc.requestType, msgs)
			if tc.wantFallback {
				assert.Equal(t, "{}", body)
			} else {
				for _, s := range tc.wantContains {
					assert.Contains(t, body, s)
				}
			}
		})
	}
}
