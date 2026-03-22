# a-kit

A Go project scaffolding CLI that generates clean-architecture Go services from protobuf definitions. No `protoc` required.

## Installation

```bash
go install github.com/jacksonfernando/a-kit@latest
```

Requires Go 1.21+. The binary is placed in `$GOPATH/bin` (make sure it's on your `$PATH`).

---

## Commands

### `a-kit create <project-name>`

Scaffolds a new Go project in a new directory.

```bash
a-kit create my-service

# With a custom Go module path
a-kit create my-service --module github.com/myorg/my-service
```

**Flags**

| Flag | Shorthand | Default | Description |
|------|-----------|---------|-------------|
| `--module` | `-m` | `<project-name>` | Go module path written into `go.mod` |

**What gets created**

```
my-service/
├── main.go                          # Echo server + dependency wiring
├── go.mod
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── Makefile
│
├── api/
│   └── example.proto                # ← source of truth for modules
│
├── global/                          # shared config, errors, response types
│   ├── configuration.go
│   ├── errors.go
│   └── global.go
│
├── middlewares/
│   └── middleware.go                # CORS + JWT validation
│
├── models/
│   └── example_dto.go               # generated from api/example.proto
│
├── utils/
│   ├── common/                      # uuid, helpers
│   ├── token/                       # JWT generate/parse
│   └── validator/                   # struct validation
│
├── mysql/                           # sqitch migration placeholders
│   ├── sqitch.conf
│   ├── sqitch.plan
│   ├── deploy/
│   ├── init/
│   ├── revert/
│   └── verify/
│
└── example/                         # generated from api/example.proto
    ├── interface.go
    ├── handler/http/
    ├── service/
    ├── repository/mysql/
    └── _mock/
```

After creation:

```bash
cd my-service
cp .env.example .env   # fill in your config
go mod tidy
go run main.go
```

---

### `a-kit generate [module-name]`

Reads `.proto` files from the `api/` directory and regenerates Go code for each module. Run this **inside your project directory** (where `go.mod` lives).

```bash
# Regenerate all modules from api/*.proto
a-kit generate

# Regenerate a single module from api/order.proto
a-kit generate order
```

**What gets generated per module**

| File | Description |
|------|-------------|
| `<module>/interface.go` | Repository + service interfaces |
| `<module>/handler/http/<module>_handler.go` | Echo HTTP handler with routes |
| `<module>/service/<module>_service.go` | Service layer (calls repository) |
| `<module>/repository/mysql/<module>_repository.go` | MySQL/GORM repository |
| `<module>/_mock/<module>_repository_mock.go` | Testify mock for repository |
| `<module>/_mock/<module>_service_mock.go` | Testify mock for service |
| `models/<module>_dto.go` | Request/response structs |

**HTTP method inference from RPC name**

| RPC prefix | HTTP method | Route |
|------------|-------------|-------|
| `Create*` | `POST` | `/v1/<modules>` |
| `List*` | `GET` | `/v1/<modules>` |
| `Get*` | `GET` | `/v1/<modules>/:id` |
| `Update*` | `PUT` | `/v1/<modules>/:id` |
| `Delete*` | `DELETE` | `/v1/<modules>/:id` |
| anything else | `POST` | `/v1/<rpc-name>` |

---

## Adding a New Module

1. Create `api/<module>.proto` defining your service and messages:

```protobuf
syntax = "proto3";

package order;

service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc GetOrder(GetOrderRequest)       returns (GetOrderResponse);
  rpc ListOrder(ListOrderRequest)     returns (ListOrderResponse);
}

message CreateOrderRequest {
  string customer_id = 1;
  double amount      = 2;
}

message CreateOrderResponse {
  string id          = 1;
  string customer_id = 2;
  double amount      = 3;
}

message GetOrderRequest {
  string id = 1;
}

message GetOrderResponse {
  string id          = 1;
  string customer_id = 2;
  double amount      = 3;
}

message ListOrderRequest {
  int32 page      = 1;
  int32 page_size = 2;
}

message ListOrderResponse {
  repeated GetOrderResponse items = 1;
  int32 total                     = 2;
}
```

2. Generate the module:

```bash
a-kit generate order
```

3. Wire the module in `main.go`:

```go
orderRepo := orderRepository.NewOrderServiceMySQLRepository(mysqlDb)
orderSvc  := orderService.NewOrderService(orderRepo)
orderHTTPHandler.NewOrderServiceHandler(e, orderSvc, mw)
```

4. Implement the repository logic in `order/repository/mysql/order_repository.go`.

---

## Proto Field Type Mapping

| Proto type | Go type |
|------------|---------|
| `string` | `string` |
| `int32` | `int32` |
| `int64` | `int64` |
| `uint32` | `uint32` |
| `uint64` | `uint64` |
| `float` | `float32` |
| `double` | `float64` |
| `bool` | `bool` |
| `bytes` | `[]byte` |
| `repeated T` | `[]T` |
| message type | `*MessageType` |

---

## Generated Project — Make Commands

| Command | Description |
|---------|-------------|
| `make run` | Run the server |
| `make build` | Build binary |
| `make test` | Run all tests |
| `make tidy` | Tidy Go modules |
| `make docker-up` | Start with docker-compose |
| `make docker-down` | Stop docker-compose |
