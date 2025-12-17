## **Recommended Project Structure**

```
project-name/
├── cmd/                          # Application entry points
│   ├── api/                      # HTTP API server entry
│   │   └── main.go
│   ├── worker/                   # Background worker entry
│   │   └── main.go
│   └── cli/                      # CLI commands entry
│       └── main.go
│
├── configs/                      # Configuration files
│   ├── config.yaml
│   ├── config.dev.yaml
│   └── config.prod.yaml
│
├── deployments/                  # Deployment configurations
│   ├── docker-compose.yml
│   ├── Dockerfile
│   └── kubernetes/
│
├── docs/                         # API documentation
│   └── openapi/
│
├── scripts/                      # Build and utility scripts
│   └── migrate.sh
│
├── internal/                     # Private application code
│   ├── core/                     # Domain Layer (innermost - pure business logic)
│   │   ├── domain/               # Business entities and value objects
│   │   │   ├── user.go
│   │   │   ├── order.go
│   │   │   └── vo/               # Value objects
│   │   │       └── email.go
│   │   │
│   │   ├── port/                 # Ports (interfaces)
│   │   │   ├── inbound/          # Input ports (driven by external actors)
│   │   │   │   └── user_service.go
│   │   │   └── outbound/         # Output ports (drive external systems)
│   │   │       └── user_repository.go
│   │   │
│   │   └── service/              # Domain services (business logic)
│   │       └── user_service.go
│   │
│   ├── application/              # Application Layer (use cases/orchestration)
│   │   ├── command/              # Write operations (CQRS)
│   │   │   └── create_user.go
│   │   ├── query/                # Read operations (CQRS)
│   │   │   └── get_user.go
│   │   └── workflow/             # Complex business workflows
│   │       └── order_processing.go
│   │
│   ├── adapter/                  # Adapter Layer (infrastructure)
│   │   ├── inbound/              # Input adapters
│   │   │   ├── http/             # HTTP handlers
│   │   │   │   ├── handler.go
│   │   │   │   └── middleware/
│   │   │   ├── grpc/             # gRPC handlers
│   │   │   └── command/          # CLI/command handlers
│   │   │
│   │   └── outbound/             # Output adapters
│   │       ├── repository/       # Data persistence
│   │       │   ├── postgres/
│   │       │   │   └── user_repository.go
│   │       │   └── redis/
│   │       │       └── cache_repository.go
│   │       ├── messaging/        # Message publishers
│   │       │   └── rabbitmq/
│   │       └── external/         # External API clients
│   │           └── payment/
│   │               └── stripe_client.go
│   │
│   ├── infrastructure/           # Cross-cutting infrastructure
│   │   ├── config/              # Configuration management
│   │   ├── database/            # Database connections
│   │   ├── server/              # HTTP/gRPC server setup
│   │   ├── telemetry/           # Logging, metrics, tracing
│   │   └── di/                  # Dependency injection (Wire)
│   │       └── wire.go
│   │
│   └── shared/                   # Shared utilities (use sparingly)
│       ├── errors/
│       ├── validator/
│       └── uuid/
│
├── migrations/                   # Database migrations
│   ├── 001_create_users_table.sql
│   └── 002_create_orders_table.sql
│
├── test/                         # Integration tests
│   ├── fixtures/
│   └── integration/
│       └── user_test.go
│
├── pkg/                          # Public libraries (optional, rare)
│   └── errors/                   # *Only* if reusable across projects
│
├── go.mod & go.sum
└── README.md
```

---

## **What Each Component Does**

### **Root Level**

**`cmd/`**  
Contains **multiple entry points** for different deployment modes. Each subdirectory is a separate binary:
- `api/main.go` - HTTP/gRPC server
- `worker/main.go` - Background job processor
- `cli/main.go` - Administrative CLI tool

**Why**: Follows Go convention (`cmd/`). Enables the same core logic to be exposed via different interfaces (REST, async workers, CLI).

---

**`configs/`**  
Environment-specific configuration files managed by `viper` or similar.

**Why**: Separates config from code. Supports 12-factor app principles.

---

**`internal/`**  
**Critical**: Contains **all private application code** that cannot be imported by external projects. This enforces architectural boundaries.

**Why**: Go's `internal/` package protection. Prevents accidental coupling.

---

### **Core Layer (`internal/core/`)**

**`domain/`**  
**Pure business entities** with zero external dependencies. Contains:
- Entities: `User`, `Order`
- Value Objects: `Email`, `Money`, `UUID`
- Aggregates: Root entities with consistency boundaries
- Business invariants and validation rules

**Why**: Heart of hexagonal architecture. Survives framework/database changes. 100% unit-testable.

---

**`port/`**  
**Interfaces (contracts)** that define how layers communicate:
- **Inbound ports**: What the application can *do* (e.g., `UserService` interface)
- **Outbound ports**: What the application needs *from the outside* (e.g., `UserRepository`, `Cache`, `MessagePublisher`)

**Why**: Dependency Inversion Principle. Core defines contracts; adapters implement them.

---

**`service/`**  
**Domain services** containing pure business logic that doesn't fit naturally in entities. Stateless operations that enforce business rules.

**Why**: Keeps entities focused. Handles cross-entity business rules.

---

### **Application Layer (`internal/application/`)**

Orchestrates the domain layer to accomplish **use cases**. Implements inbound ports.
- **Command**: Write operations (Create, Update, Delete)
- **Query**: Read operations (Get, List)
- **Workflow**: Complex multi-step processes (e.g., "Place Order" = Validate → Reserve Inventory → Charge Payment → Create Order)

**Why**: Separates "what" (domain) from "how" (use case flow). Enables CQRS naturally.

---

### **Adapter Layer (`internal/adapter/`)**

**`inbound/`** - **Input adapters** that translate external requests into calls to the application layer:
- `http/`: Gin/Fiber handlers that parse requests and call use cases
- `grpc/`: gRPC service implementations
- `command/`: CLI command handlers

**Why**: Framework isolation. Can swap HTTP framework without touching business logic.

---

**`outbound/`** - **Output adapters** that implement outbound ports:
- `repository/`: Database implementations (PostgreSQL, MongoDB)
- `messaging/`: Message queue publishers (RabbitMQ, Kafka)
- `external/`: HTTP clients for third-party APIs (Stripe, SendGrid)

**Why**: Infrastructure pluggability. Can change PostgreSQL → MongoDB by swapping one adapter.

---

### **Infrastructure Layer (`internal/infrastructure/`)**

**`config/`** - Configuration loading and validation  
**`database/`** - Connection pooling, migrations  
**`server/`** - HTTP/gRPC server lifecycle management  
**`telemetry/`** - Structured logging (zerolog), metrics (Prometheus), tracing (OpenTelemetry)  
**`di/`** - **Google Wire** dependency injection setup

**Why**: Cross-cutting concerns that don't belong in adapters. Single source of truth for infra setup.

---

## **State-of-the-Art Patterns**

### **1. Multiple Entry Points**
```go
// cmd/api/main.go - HTTP server
func main() { wire.Build(mustInitializeServer()) }

// cmd/worker/main.go - Background worker
func main() { wire.Build(mustInitializeWorker()) }
```

### **2. Compile-Time DI with Google Wire**
```go
// infrastructure/di/wire.go
func InitializeUserService() (*application.UserService, error) {
    wire.Build(
        adapter.outbound.repository.NewPostgresUserRepository,
        adapter.inbound.http.NewUserHandler,
        application.NewUserService,
    )
    return &application.UserService{}, nil
}
```
**Why**: Zero runtime overhead, compile-time safety, explicit dependencies.

### **3. Feature-Based Organization (Scaling Pattern)**
For large monoliths, nest features **within** layers:
```
internal/core/domain/user/
internal/core/domain/order/
internal/application/user/
internal/application/order/
```
**Why**: Better code discoverability while preserving layer boundaries.

### **4. Automated Architecture Testing**
```go
// test/architecture_test.go
func TestArchitecture(t *testing.T) {
    // Ensure domain doesn't import adapters
    // Ensure application only imports domain
    // using arch-go or go-arch-lint
}
```

---

## **Why This Structure?**

| Benefit | Implementation |
|---------|----------------|
| **Framework Agnostic** | Domain/core has zero external dependencies |
| **Testable** | Unit test domain without DB; use mocks for ports |
| **Scalable** | Feature-based organization within layers |
| **Pluggable** | Swap adapters without changing core |
| **Maintainable** | Clear boundaries; new devs know where to look |
| **Cloud-Native** | Multiple binaries (`cmd/*`) for different scaling needs |
