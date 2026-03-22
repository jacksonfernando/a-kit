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
├── main.go
├── go.mod
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── Makefile
│
├── api/
│   └── example.proto            # source of truth — edit this, then run a-kit generate
│
├── global/                      # shared config, errors, response types
├── middlewares/                 # CORS + JWT validation
├── models/                      # generated DTOs (from proto messages)
├── utils/                       # common, token, validator
├── mysql/                       # sqitch migration placeholders
│
├── example/                     # generated from non-Internal RPCs
│   ├── interface.go
│   ├── handler/http/
│   ├── service/
│   ├── repository/mysql/
│   └── _mock/
│
└── internal/
    └── example/                 # generated from Internal RPCs (no HTTP layer)
        ├── interface.go
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

---

## Proto File Syntax

Each module has one proto file at `api/<module>.proto`.

### RPC Modifiers

| Modifier | Generated in | HTTP handler |
|----------|-------------|--------------|
| _(none)_ | `<module>/` | ✅ Yes |
| `Internal` | `internal/<module>/` | ❌ No |

```protobuf
syntax = "proto3";

package order;

service OrderService {
  // Public — exposed as HTTP endpoints in order/
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc GetOrder(GetOrderRequest)       returns (GetOrderResponse);
  rpc ListOrder(ListOrderRequest)     returns (ListOrderResponse);

  // Internal — domain logic only, placed in internal/order/, no HTTP route
  rpc RecalculateInterest(RecalculateInterestRequest) returns (RecalculateInterestResponse) Internal;
  rpc MarkOverdue(MarkOverdueRequest)                 returns (MarkOverdueResponse) Internal;
}
```

### HTTP Method Inference

| RPC name prefix | HTTP method | Route |
|-----------------|-------------|-------|
| `Create*` | `POST` | `/v1/<modules>` |
| `List*` | `GET` | `/v1/<modules>` |
| `Get*` | `GET` | `/v1/<modules>/:id` |
| `Update*` | `PUT` | `/v1/<modules>/:id` |
| `Delete*` | `DELETE` | `/v1/<modules>/:id` |
| anything else | `POST` | `/v1/<rpc-name>` |

### Proto → Go Type Mapping

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

## Adding a New Module

1. Create `api/<module>.proto`:

```protobuf
syntax = "proto3";

package payment;

service PaymentService {
  rpc CreatePayment(CreatePaymentRequest)   returns (CreatePaymentResponse);
  rpc GetPayment(GetPaymentRequest)         returns (GetPaymentResponse);

  rpc ProcessRefund(ProcessRefundRequest)   returns (ProcessRefundResponse) Internal;
}

message CreatePaymentRequest {
  string order_id = 1;
  double amount   = 2;
}

message CreatePaymentResponse {
  string id       = 1;
  string order_id = 2;
  double amount   = 3;
}

message GetPaymentRequest {
  string id = 1;
}

message GetPaymentResponse {
  string id       = 1;
  string order_id = 2;
  double amount   = 3;
}

message ProcessRefundRequest {
  string payment_id = 1;
  double amount     = 2;
}

message ProcessRefundResponse {
  bool success = 1;
}
```

2. Generate the module:

```bash
a-kit generate payment
```

3. Wire external routes in `main.go`:

```go
paymentRepo := paymentRepository.NewPaymentServiceMySQLRepository(mysqlDb)
paymentSvc  := paymentService.NewPaymentService(paymentRepo)
paymentHTTPHandler.NewPaymentServiceHandler(e, paymentSvc, mw)
```

4. To use internal domain logic from another module:

```go
import internalPayment "my-service/internal/payment"
```

5. Implement repository logic in `payment/repository/mysql/` and `internal/payment/repository/mysql/`.

---

## What Gets Generated per Module

### External RPCs → `<module>/`

| File | Description |
|------|-------------|
| `<module>/interface.go` | Repository + service interfaces |
| `<module>/handler/http/<module>_handler.go` | Echo HTTP handler with routes |
| `<module>/service/<module>_service.go` | Service layer |
| `<module>/repository/mysql/<module>_repository.go` | MySQL/GORM repository stub |
| `<module>/_mock/<module>_repository_mock.go` | Testify mock for repository |
| `<module>/_mock/<module>_service_mock.go` | Testify mock for service |
| `models/<module>_dto.go` | All request/response structs |

### Internal RPCs → `internal/<module>/`

| File | Description |
|------|-------------|
| `internal/<module>/interface.go` | Repository + service interfaces |
| `internal/<module>/service/<module>_service.go` | Service layer |
| `internal/<module>/repository/mysql/<module>_repository.go` | MySQL/GORM repository stub |
| `internal/<module>/_mock/<module>_repository_mock.go` | Testify mock |
| `internal/<module>/_mock/<module>_service_mock.go` | Testify mock |

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
