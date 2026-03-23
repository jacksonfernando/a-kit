package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ParseProto ───────────────────────────────────────────────────────────────

func TestParseProto_Package(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", `package foo;`, "foo"},
		{"dotted", `package order.v1;`, "order.v1"},
		{"empty file", ``, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pf, err := ParseProto(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, pf.Package)
		})
	}
}

func TestParseProto_Messages(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantMsgs   int
		firstName  string
		fieldCount int
		firstField FieldDef
	}{
		{
			name: "single message single field",
			input: `message GetOrderRequest {
  string name = 1;
}`,
			wantMsgs:   1,
			firstName:  "GetOrderRequest",
			fieldCount: 1,
			firstField: FieldDef{Name: "name", Type: "string", Number: 1},
		},
		{
			name: "repeated field",
			input: `message ListResponse {
  repeated string items = 1;
  int32 total = 2;
}`,
			wantMsgs:   1,
			firstName:  "ListResponse",
			fieldCount: 2,
			firstField: FieldDef{Name: "items", Type: "string", Number: 1, Repeated: true},
		},
		{
			name: "qualified type (FieldMask)",
			input: `message UpdateRequest {
  google.protobuf.FieldMask update_mask = 1;
}`,
			wantMsgs:   1,
			firstName:  "UpdateRequest",
			fieldCount: 1,
			firstField: FieldDef{Name: "update_mask", Type: "google.protobuf.FieldMask", Number: 1},
		},
		{
			name: "multiple messages",
			input: `message Foo { string a = 1; }
message Bar { int32 b = 1; }`,
			wantMsgs:  2,
			firstName: "Foo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pf, err := ParseProto(tc.input)
			require.NoError(t, err)
			require.Len(t, pf.Messages, tc.wantMsgs)
			assert.Equal(t, tc.firstName, pf.Messages[0].Name)
			if tc.fieldCount > 0 {
				assert.Len(t, pf.Messages[0].Fields, tc.fieldCount)
			}
			if tc.firstField.Name != "" {
				assert.Equal(t, tc.firstField, pf.Messages[0].Fields[0])
			}
		})
	}
}

func TestParseProto_RPC_InlineAnnotation(t *testing.T) {
	input := `
service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse) POST /v1/orders;
  rpc GetOrder(GetOrderRequest) returns (Order) GET /v1/orders/:id;
  rpc UpdateOrder(UpdateOrderRequest) returns (Order) PATCH /v1/orders/:id;
  rpc DeleteOrder(DeleteOrderRequest) returns (Order) DELETE /v1/orders/:id;
}`
	pf, err := ParseProto(input)
	require.NoError(t, err)
	require.Len(t, pf.Services, 1)

	rpcs := pf.Services[0].RPCs
	require.Len(t, rpcs, 4)

	cases := []struct {
		method HTTPMethod
		path   string
	}{
		{HTTPMethodPOST, "/v1/orders"},
		{HTTPMethodGET, "/v1/orders/:id"},
		{HTTPMethodPATCH, "/v1/orders/:id"},
		{HTTPMethodDELETE, "/v1/orders/:id"},
	}
	for i, tc := range cases {
		assert.Equal(t, tc.method, rpcs[i].HTTPMethod, "rpc %d method", i)
		assert.Equal(t, tc.path, rpcs[i].HTTPPath, "rpc %d path", i)
	}
}

func TestParseProto_RPC_GoogleHTTP(t *testing.T) {
	input := `
service ExampleService {
  rpc GetExample(GetExampleRequest) returns (Example) {
    option (google.api.http) = {
      get: "/v1/{name=examples/*}"
    };
  }

  rpc CreateExample(CreateExampleRequest) returns (Example) {
    option (google.api.http) = {
      post: "/v1/examples"
      body: "example"
    };
  }

  rpc UpdateExample(UpdateExampleRequest) returns (Example) {
    option (google.api.http) = {
      patch: "/v1/{example.name=examples/*}"
      body:  "example"
    };
  }

  rpc DeleteExample(DeleteExampleRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = { delete: "/v1/{name=examples/*}" };
  }

  rpc SearchExamples(SearchExamplesRequest) returns (SearchExamplesResponse) {
    option (google.api.http) = {
      post: "/v1/examples:search"
      body: "*"
    };
  }
}`

	pf, err := ParseProto(input)
	require.NoError(t, err)
	require.Len(t, pf.Services, 1)

	rpcs := pf.Services[0].RPCs
	require.Len(t, rpcs, 5)

	cases := []struct {
		name     string
		method   HTTPMethod
		path     string
		body     string
		respType string
	}{
		{"GetExample", HTTPMethodGET, "/v1/{name=examples/*}", "", "Example"},
		{"CreateExample", HTTPMethodPOST, "/v1/examples", "example", "Example"},
		{"UpdateExample", HTTPMethodPATCH, "/v1/{example.name=examples/*}", "example", "Example"},
		{"DeleteExample", HTTPMethodDELETE, "/v1/{name=examples/*}", "", "Empty"},
		{"SearchExamples", HTTPMethodPOST, "/v1/examples:search", "*", "SearchExamplesResponse"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.name, rpcs[i].Name)
			assert.Equal(t, tc.method, rpcs[i].HTTPMethod)
			assert.Equal(t, tc.path, rpcs[i].HTTPPath)
			assert.Equal(t, tc.body, rpcs[i].HTTPBody)
			assert.Equal(t, tc.respType, rpcs[i].ResponseType)
		})
	}
}

func TestParseProto_RPC_Internal(t *testing.T) {
	input := `
service OrderService {
  rpc GetOrder(GetOrderRequest) returns (Order) {
    option (google.api.http) = { get: "/v1/{name=orders/*}" };
  }
  rpc RecalculateTax(RecalculateTaxRequest) returns (RecalculateTaxResponse) Internal;
}`
	pf, err := ParseProto(input)
	require.NoError(t, err)
	rpcs := pf.Services[0].RPCs
	require.Len(t, rpcs, 2)

	assert.False(t, rpcs[0].Internal, "GetOrder should not be internal")
	assert.True(t, rpcs[1].Internal, "RecalculateTax should be internal")
}

func TestParseProto_NormalizeType(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"google.protobuf.Empty", "Empty"},
		{"Order", "Order"},
		{"ListOrdersResponse", "ListOrdersResponse"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeType(tc.input))
		})
	}
}

func TestParseProto_FullFile(t *testing.T) {
	input := `
syntax = "proto3";
package order.v1;

message Order {
  string name         = 1;
  string display_name = 2;
  double amount       = 3;
}

message GetOrderRequest { string name = 1; }

message ListOrdersRequest {
  int32  page_size  = 1;
  string page_token = 2;
  string filter     = 3;
}

service OrderService {
  rpc GetOrder(GetOrderRequest) returns (Order) {
    option (google.api.http) = { get: "/v1/{name=orders/*}" };
  }
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse) {
    option (google.api.http) = { get: "/v1/orders" };
  }
  rpc ProcessRefund(ProcessRefundRequest) returns (ProcessRefundResponse) Internal;
}`

	pf, err := ParseProto(input)
	require.NoError(t, err)

	assert.Equal(t, "order.v1", pf.Package)
	assert.Len(t, pf.Messages, 3)
	assert.Len(t, pf.Services, 1)
	assert.Equal(t, "OrderService", pf.Services[0].Name)

	rpcs := pf.Services[0].RPCs
	assert.Len(t, rpcs, 3)
	assert.False(t, rpcs[0].Internal)
	assert.False(t, rpcs[1].Internal)
	assert.True(t, rpcs[2].Internal)
}
