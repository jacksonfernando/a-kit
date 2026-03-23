```
  ██████╗       ██╗  ██╗██╗████████╗
  ██╔══██╗      ██║ ██╔╝██║╚══██╔══╝
  ███████║█████╗█████╔╝ ██║   ██║
  ██╔══██║╚════╝██╔═██╗ ██║   ██║
  ██║  ██║      ██║  ██╗██║   ██║
  ╚═╝  ╚═╝      ╚═╝  ╚═╝╚═╝   ╚═╝

```

A CLI that scaffolds production-ready Go microservices from **protobuf definitions** (Google API style). No `protoc` required.

---

## Installation

```bash
go install github.com/jacksonfernando/a-kit@latest
```

Requires Go 1.21+. The binary lands in `$GOPATH/bin` — make sure it's on your `$PATH`.

### Build from source

```bash
git clone https://github.com/jacksonfernando/a-kit.git
cd a-kit
make install          # installs with version injected via ldflags
# or
make build            # produces bin/a-kit
```

---

## Commands

### `a-kit create <project-name>`

Scaffolds a new Go project and generates an example module from the bundled `api/example.proto`.

```bash
a-kit create my-service

# With an explicit Go module path
a-kit create my-service --module github.com/myorg/my-service
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--module` | `-m` | `<project-name>` | Go module path written into `go.mod` |

**Generated layout**

```
my-service/
├── main.go
├── go.mod
├── config.yaml              # app config (supports {{VAR:default}} env injection)
├── config.json.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── Makefile
│
├── api/
│   └── example.proto        # source of truth — edit this, then run a-kit generate
│
├── global/                  # shared config, error types, response envelopes
├── middlewares/             # CORS + JWT middleware
├── models/                  # generated DTOs (from proto messages)
│   └── example_dto.go
├── utils/                   # config loader, validator, token, common helpers
├── mysql/                   # sqitch migration stubs
│
├── example/                 # generated from public RPCs
│   ├── interface.go
│   ├── handler/http/
│   │   ├── example_handler.go
│   │   └── example_handler_test.go   ← generated unit tests
│   ├── service/
│   │   ├── example_service.go
│   │   └── example_service_test.go   ← generated unit tests
│   ├── repository/mysql/
│   └── _mock/
│
└── internal/
    └── example/             # generated from Internal RPCs (no HTTP layer)
        ├── interface.go
        ├── service/
        ├── repository/mysql/
        └── _mock/
```

After creation:

```bash
cd my-service
go mod tidy
go run main.go
```

---

### `a-kit generate [module-name]`

Re-generates Go code from `.proto` files inside `api/`. Run this **inside your project directory**.

```bash
# Regenerate all modules from api/*.proto
a-kit generate

# Regenerate a single module
a-kit generate order
```

---

### `a-kit version`

```bash
a-kit version
# a-kit v1.2.0
```

---

### `a-kit help`

```bash
a-kit help
a-kit help create
a-kit help generate
```

---

## Proto File Syntax

Each module has one file at `api/<module>.proto`. a-kit follows the **[Google API Design Guide](https://cloud.google.com/apis/design)** (AIP / resource-oriented design).

### Full example

```protobuf
syntax = "proto3";

package order.v1;

import "google/api/annotations.proto";
import "google/protobuf/field_mask.proto";
import "google/protobuf/empty.proto";

// Order is the resource managed by this service.
message Order {
  string name         = 1;  // resource name, e.g. "orders/123"
  string display_name = 2;
  string customer_id  = 3;
  double amount       = 4;
}

service OrderService {

  // Standard GET by resource name — path param from URL
  rpc GetOrder(GetOrderRequest) returns (Order) {
    option (google.api.http) = {
      get: "/v1/{name=orders/*}"
    };
  }

  // List with query params (page_size, filter, order_by)
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse) {
    option (google.api.http) = {
      get: "/v1/orders"
    };
  }

  // Create — body field name matches the message field
  rpc CreateOrder(CreateOrderRequest) returns (Order) {
    option (google.api.http) = {
      post: "/v1/orders"
      body: "order"
    };
  }

  // Partial update via field_mask
  rpc UpdateOrder(UpdateOrderRequest) returns (Order) {
    option (google.api.http) = {
      patch: "/v1/{order.name=orders/*}"
      body: "order"
    };
  }

  // Delete returns Empty
  rpc DeleteOrder(DeleteOrderRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      delete: "/v1/{name=orders/*}"
    };
  }

  // Custom method — colon suffix becomes a sub-path segment
  rpc CancelOrder(CancelOrderRequest) returns (Order) {
    option (google.api.http) = {
      post: "/v1/{name=orders/*}:cancel"
      body: "*"
    };
  }

  // Internal — domain logic only, no HTTP route, placed in internal/order/
  rpc RecalculateTax(RecalculateTaxRequest) returns (RecalculateTaxResponse) Internal;
}

// ── Standard messages ────────────────────────────────────────────────────────

message GetOrderRequest    { string name = 1; }

message ListOrdersRequest {
  int32  page_size  = 1;
  string page_token = 2;
  string filter     = 3;
  string order_by   = 4;
}

message ListOrdersResponse {
  repeated Order orders         = 1;
  string         next_page_token = 2;
  int32          total_size      = 3;
}

message CreateOrderRequest { Order order = 1; }

message UpdateOrderRequest {
  Order                    order       = 1;
  google.protobuf.FieldMask update_mask = 2;
}

message DeleteOrderRequest { string name = 1; }

message CancelOrderRequest {
  string name   = 1;
  string reason = 2;
}

// ── Internal messages ────────────────────────────────────────────────────────

message RecalculateTaxRequest  { string name = 1; }
message RecalculateTaxResponse { bool success = 1; }
```

---

### HTTP option block

```protobuf
option (google.api.http) = {
  get | post | put | patch | delete: "<path-template>"
  body: "<field-name> | *"    // only for POST / PUT / PATCH
};
```

### Path template → Echo route

| Path template | Echo route | Binding |
|---|---|---|
| `/v1/{name=orders/*}` | `/v1/orders/:name` | `req.Name = c.Param("name")` |
| `/v1/{order.name=orders/*}` | `/v1/orders/:name` | _(TODO comment — nested field)_ |
| `/v1/orders:search` | `/v1/orders:search` | custom method literal |
| `/v1/{name=orders/*}:cancel` | `/v1/orders/:name/cancel` | path param + custom method |

### Query parameters (GET / DELETE)

For GET and DELETE requests, all request message fields that are **not** path params are bound from the query string via Echo's `Bind()`. The models include `query:"<field>"` struct tags automatically:

```go
type ListOrdersRequest struct {
    PageSize  int32  `json:"page_size"  query:"page_size"`
    PageToken string `json:"page_token" query:"page_token"`
    Filter    string `json:"filter"     query:"filter"`
    OrderBy   string `json:"order_by"   query:"order_by"`
}
```

A request to `GET /v1/orders?page_size=20&filter=status=ACTIVE` populates those fields automatically.

---

### `Internal` keyword

RPCs marked `Internal` are placed under `internal/<module>/` (Go's built-in access boundary) and have **no HTTP handler**.

```protobuf
rpc RecalculateTax(RecalculateTaxRequest) returns (RecalculateTaxResponse) Internal;
```

---

### Inline HTTP annotations (simple alternative)

For simple cases you can use the shorthand inline annotation instead of the full option block:

```protobuf
rpc GetOrder(GetOrderRequest) returns (Order) GET /v1/orders/:id;
rpc CreateOrder(CreateOrderRequest) returns (Order) POST /v1/orders;
rpc UpdateOrder(UpdateOrderRequest) returns (Order) PATCH /v1/orders/:id;
rpc DeleteOrder(DeleteOrderRequest) returns (google.protobuf.Empty) DELETE /v1/orders/:id;
```

### HTTP method inference (fallback)

If no annotation is provided, the method is inferred from the RPC name prefix:

| RPC name prefix | HTTP method | Route |
|---|---|---|
| `Create*` | `POST` | `/v1/<modules>` |
| `List*` | `GET` | `/v1/<modules>` |
| `Get*` | `GET` | `/v1/<modules>/:id` |
| `Update*` | `PATCH` | `/v1/<modules>/:id` |
| `Delete*` | `DELETE` | `/v1/<modules>/:id` |
| anything else | `POST` | `/v1/<rpc-name>` |

---

### Proto → Go type mapping

| Proto type | Go type | Notes |
|---|---|---|
| `string` | `string` | |
| `int32` | `int32` | |
| `int64` | `int64` | |
| `uint32` | `uint32` | |
| `uint64` | `uint64` | |
| `float` | `float32` | |
| `double` | `float64` | |
| `bool` | `bool` | |
| `bytes` | `[]byte` | |
| `repeated T` | `[]T` / `[]*T` | |
| message type | `*MessageType` | |
| `google.protobuf.FieldMask` | `[]string` | list of field paths |
| `google.protobuf.Empty` | `struct{}` / `models.Empty` | empty response |

---

## What gets generated per module

### Public RPCs → `<module>/`

| File | Description |
|---|---|
| `<module>/interface.go` | Repository + service interfaces |
| `<module>/handler/http/<module>_handler.go` | Echo HTTP handler with all routes |
| `<module>/handler/http/<module>_handler_test.go` | Unit tests for every endpoint |
| `<module>/service/<module>_service.go` | Service layer |
| `<module>/service/<module>_service_test.go` | Unit tests using mock repository |
| `<module>/repository/mysql/<module>_repository.go` | MySQL/GORM repository stub |
| `<module>/_mock/<module>_repository_mock.go` | Testify mock for repository |
| `<module>/_mock/<module>_service_mock.go` | Testify mock for service |
| `models/<module>_dto.go` | All request/response/resource structs |

### Internal RPCs → `internal/<module>/`

| File | Description |
|---|---|
| `internal/<module>/interface.go` | Repository + service interfaces |
| `internal/<module>/service/<module>_service.go` | Service layer |
| `internal/<module>/repository/mysql/<module>_repository.go` | MySQL/GORM repository stub |
| `internal/<module>/_mock/` | Testify mocks |

---

## Adding a new module

1. Create `api/order.proto` (Google API style — see example above)

2. Generate the module:

```bash
a-kit generate order
```

3. Wire routes in `main.go`:

```go
orderRepo    := orderRepository.NewOrderServiceMySQLRepository(mysqlDb)
orderSvc     := orderService.NewOrderService(orderRepo)
orderHandler.NewOrderServiceHandler(e, orderSvc, mw)
```

4. Implement the repository stub in `order/repository/mysql/order_repository.go`.

5. To use an internal RPC from another module:

```go
import internalOrder "my-service/internal/order"
```

---

## Config file

Generated projects support `config.yaml` (default) or any JSON/YAML file pointed to by `APP_CONFIG`:

```yaml
app:
  port: "{{APP_PORT:9000}}"
  env:  "{{APP_ENV:development}}"

database:
  host: "{{DB_HOST:localhost}}"
  port: "{{DB_PORT:3306}}"
  name: "{{DB_NAME:mydb}}"
  user: "{{DB_USER:root}}"
  pass: "{{DB_PASS:}}"
```

`{{VAR:default}}` — reads from environment variable `VAR`, falling back to `default`.

```bash
APP_CONFIG=config.prod.yaml go run main.go
```

---

## Generated project — Make targets

| Command | Description |
|---|---|
| `make run` | Run the server |
| `make build` | Build binary |
| `make test` | Run all tests |
| `make tidy` | Tidy Go modules |
| `make docker-up` | Start with docker-compose |
| `make docker-down` | Stop docker-compose |

