# Forge Orchestrator - demo purposes 

Forge Orchestrator is a Go microservice that lets you manage video production equipment (cameras, encoders) from different vendors (Sony) through a single unified API. Instead of learning each vendor's different API format, authentication method, and data structure, you just send one simple request to your server (port 8080) saying "create a Sony camera named cam-1 in 4K", and the system automatically translates your request into Sony's specific format, sends it to their API, gets the response, translates it back into your format, and returns it to you - making it easy to add new vendors later by just writing one new "translator" file without changing any other code.

---

## 📋 Table of Contents
- [Overview](#overview)
- [The Problem](#the-problem)
- [The Solution](#the-solution)
- [Architecture](#architecture)
- [Technical Stack](#technical-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [API Endpoints](#api-endpoints)
- [How It Works](#how-it-works)
- [Learning Objectives](#learning-objectives)

---

## 🎯 Overview

**Forge Orchestrator** is a microservice-based platform that provides a unified API for managing resources across multiple vendor systems. It abstracts vendor-specific implementations behind a common interface, enabling organizations to integrate new hardware vendors without rewriting application code.

This project was built to understand the architectural patterns used by NBCUniversal's **Production Application Engineering** team to support their **Virtual Production Control Room (VPCR)** platform, which powers live news production across dozens of markets.

---

## ❌ The Problem

Modern media production requires integrating hardware from multiple vendors:

| Vendor | Equipment | API Format | Authentication |
|--------|-----------|------------|----------------|
| **Sony** | Professional cameras | REST + JSON | OAuth 2.0 |
| **AWS** | Cloud video processing | SDK + Protobuf | IAM credentials |
| **Blackmagic** | Video encoders | gRPC | API keys |
| **Evertz** | Routing switches | SOAP + XML | Basic Auth |

**Without abstraction**, each integration requires:
- Custom code for each vendor's API
- Vendor-specific authentication handling
- Different error handling patterns
- Months of development time per vendor

**Result:** Engineering bottlenecks, high maintenance costs, and slow time-to-market.

---

## ✅ The Solution

**Forge** provides a **single unified API** that abstracts all vendor integrations:

```
┌─────────────────────────────────────────────────────┐
│         Internal Applications (VPCR, etc.)          │
│                                                     │
│  "Create a Sony camera in 4K resolution"           │
└────────────────────┬────────────────────────────────┘
                     │ Single standardized API
                     ↓
┌─────────────────────────────────────────────────────┐
│              FORGE CONTROLLER                       │
│  ┌─────────────────────────────────────────────┐   │
│  │  Routes request to appropriate provider     │   │
│  └─────────────────────────────────────────────┘   │
└────┬─────────────────┬────────────────────┬─────────┘
     │                 │                    │
     ↓                 ↓                    ↓
┌──────────┐     ┌──────────┐       ┌──────────┐
│  Sony    │     │   AWS    │       │Blackmagic│
│ Provider │     │ Provider │       │ Provider │
└────┬─────┘     └────┬─────┘       └────┬─────┘
     │                │                   │
     ↓                ↓                   ↓
[Sony API]      [AWS API]         [Blackmagic API]
```

**Benefits:**
- ⚡ **Fast onboarding** - Add new vendors in days, not months
- 🔄 **Vendor agnostic** - Switch vendors without changing application code
- 🛡️ **Centralized logic** - One place for authentication, retries, monitoring
- 📈 **Scalable** - Handle hundreds of vendor integrations

---

## 🏗️ Architecture

### **System Components**

#### **1. Forge Controller** (`cmd/controller/main.go`)
- HTTP REST API server (port 8080)
- Routes requests to appropriate vendor providers
- Manages resource lifecycle (CRUD operations)
- Stores resource state in in-memory database
- Provides unified interface to internal applications

#### **2. Vendor Providers** (`pkg/provider/`)
- **Interface-based design** - All providers implement `VendorProvider` interface
- **Translation layer** - Converts standardized requests → vendor-specific formats
- **HTTP client management** - Handles authentication, timeouts, retries
- **Error normalization** - Returns consistent error formats

#### **3. Mock Vendor API** (`cmd/vendor-api/main.go`)
- Simulates Sony's production API (port 9000)
- Used for local development and testing
- Implements realistic response patterns
- No external dependencies required

### **Data Models**

```go
// NBCU's internal representation
type ForgeResource struct {
    ID        string          // Unique identifier
    Type      string          // "camera", "encoder", etc.
    Name      string          // Human-readable name
    Namespace string          // Environment (prod, staging)
    Spec      ResourceSpec    // Desired configuration
    Status    ResourceStatus  // Current state
}

// Vendor-specific format (Sony)
type SonyDeviceRequest struct {
    DeviceName string
    Model      string
    Settings   map[string]string
}
```

---

## 🛠️ Technical Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Language** | Go 1.21+ | High-performance, concurrent system programming |
| **HTTP Router** | gorilla/mux | RESTful API routing and parameter extraction |
| **Concurrency** | sync.RWMutex | Thread-safe in-memory database |
| **Context** | context.Context | Request cancellation and timeout management |
| **Testing** | go test, fuzzing | Unit tests and security testing |
| **Containerization** | Docker | Multi-stage builds for minimal images |
| **Orchestration** | Kubernetes | Deployment, scaling, health checks |

---

## 📁 Project Structure

```
forge-orchestrator/
├── cmd/
│   ├── controller/          # Main Forge Controller service
│   │   └── main.go          # HTTP server, routing, orchestration
│   └── vendor-api/          # Mock Sony API for testing
│       └── main.go          # Simulated vendor endpoints
├── pkg/
│   ├── provider/            # Vendor integration implementations
│   │   ├── interface.go     # VendorProvider contract
│   │   ├── sony_provider.go # Sony-specific translation
│   │   └── aws_provider.go  # AWS implementation (future)
│   ├── models/              # Data structures
│   │   ├── resource.go      # NBCU internal models
│   │   └── vendor.go        # Vendor-specific models
│   └── client/              # Shared utilities
│       └── http_client.go   # Retry logic, validation
├── tests/
│   ├── main_test.go         # Unit tests
│   └── fuzz_test.go         # Fuzzing for security
├── k8s/                     # Kubernetes manifests
│   ├── controller-deployment.yaml
│   ├── vendor-api-deployment.yaml
│   └── configmap.yaml
├── Dockerfile.controller    # Multi-stage build for controller
├── Dockerfile.vendor        # Multi-stage build for mock API
├── go.mod                   # Go module dependencies
└── README.md
```

---

## 🚀 Getting Started

### **Prerequisites**

```bash
# Required
- Go 1.21 or higher
- Docker (for containerization)
- curl or Postman (for API testing)

# Optional
- Kubernetes (Minikube/Kind for local deployment)
- kubectl CLI
```

### **Installation**

```bash
# Clone the repository
git clone https://github.com/yourusername/forge-orchestrator.git
cd forge-orchestrator

# Install dependencies
go mod download

# Set environment variables
export SONY_API_URL="http://localhost:9000"
export SONY_API_KEY="test-api-key"
export PORT="8080"
```

### **Running Locally**

**Terminal 1 - Start Mock Vendor API:**
```bash
go run cmd/vendor-api/main.go
# Mock Vendor API listening on :9000
```

**Terminal 2 - Start Forge Controller:**
```bash
go run cmd/controller/main.go
# Controller listening on :8080
```

### **Testing the API**

**Create a Resource:**
```bash
curl -X POST http://localhost:8080/resources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-camera-1",
    "type": "camera",
    "namespace": "production",
    "spec": {
      "vendor_type": "sony",
      "config": {
        "model": "PXW-Z450",
        "resolution": "4K",
        "frame_rate": 60
      }
    }
  }'
```

**Response:**
```json
{
  "id": "res-1706640000000",
  "name": "test-camera-1",
  "type": "camera",
  "status": {
    "phase": "Running",
    "message": "Resource created successfully",
    "vendor_id": "sony-cam-78392"
  },
  "created_at": "2026-01-30T10:00:00Z"
}
```

**Get Resource Status:**
```bash
curl http://localhost:8080/resources/res-1706640000000
```

**Delete Resource:**
```bash
curl -X DELETE http://localhost:8080/resources/res-1706640000000
```

---

## 📡 API Endpoints

### **POST /resources**
Create a new vendor resource

**Request Body:**
```json
{
  "name": "string (required)",
  "type": "string (required)",
  "namespace": "string",
  "spec": {
    "vendor_type": "string (required)",
    "config": {}
  }
}
```

**Response:** `201 Created` with full resource object

---

### **GET /resources/{id}**
Retrieve resource status

**Response:** `200 OK` with resource details
```json
{
  "id": "res-123",
  "status": {
    "phase": "Running|Pending|Failed",
    "vendor_id": "vendor-specific-id"
  }
}
```

---

### **DELETE /resources/{id}**
Remove a resource from vendor system

**Response:** `204 No Content` on success

---

## 🔄 How It Works

### **Request Flow**

```
1. Application sends standardized request
   POST /resources { "vendor_type": "sony", ... }
   
2. Controller validates and routes
   - Checks required fields
   - Selects Sony Provider
   
3. Provider translates format
   NBCU Format → Sony API Format
   
4. Provider calls vendor API
   POST https://sony-api/devices
   
5. Vendor provisions hardware
   Returns device_id and credentials
   
6. Controller stores state
   ResourceDB[id] = resource
   
7. Response returned to application
   { "id": "res-123", "status": "Running" }
```

### **The Provider Pattern**

All providers implement the same interface:

```go
type VendorProvider interface {
    Create(ctx context.Context, resource *ForgeResource) (*ResourceStatus, error)
    Read(ctx context.Context, vendorID string) (*ResourceStatus, error)
    Update(ctx context.Context, resource *ForgeResource) (*ResourceStatus, error)
    Delete(ctx context.Context, vendorID string) error
    HealthCheck(ctx context.Context) error
}
```

This allows adding new vendors without modifying the controller:

```go
// Adding a new vendor is this simple
controller.Providers["aws"] = provider.NewAWSProvider(baseURL, apiKey)
```

---

## 🎓 Learning Objectives

This project demonstrates understanding of:

### **1. Go Language Fundamentals**
- Interface-based polymorphism
- Struct composition and embedding
- Pointer receivers vs value receivers
- Error handling patterns
- JSON marshaling/unmarshaling

### **2. Concurrent Programming**
- Mutex for thread-safe operations
- Context for cancellation and timeouts
- Goroutine management

### **3. API Design**
- RESTful principles
- HTTP status codes
- Request/response patterns
- Idempotency considerations

### **4. Software Architecture**
- Provider pattern (adapter pattern)
- Dependency injection
- Separation of concerns
- Contract-driven development

### **5. Production Engineering**
- Retry logic and circuit breakers
- Health checks and observability
- Error propagation and logging
- Security (API key management)

### **6. DevOps Practices**
- Containerization with Docker
- Kubernetes deployments
- Configuration management
- Multi-stage builds

---

## 🎯 Why This Project?

This project was built to prepare for the **NBCUniversal Production Application Engineering Internship**, which focuses on:

> "The Production Application Engineering team provides services to multiple businesses within the larger NBCU Enterprise, including the architecture and deployment of internal platforms to support live news production - Virtual Production Control Room (VPCR) and NBCU Forge."

**Job Requirements:**
- ✅ **Golang proficiency** - Entire codebase in Go
- ✅ **Kubernetes familiarity** - K8s manifests and deployments
- ✅ **API consumption** - HTTP client with proper error handling
- ✅ **Crossplane providers** - Provider pattern mirrors Crossplane architecture

**Learning Approach:**
Rather than copying working code, this project provides a skeleton with `TODO` comments, forcing manual implementation of:
- JSON schema validation
- HTTP client logic
- Error propagation
- Memory management
- Nil pointer handling

This hands-on approach ensures deep understanding of Go's type system and production-grade API development.

---

## 🔮 Future Enhancements

- [ ] Add AWS MediaLive provider
- [ ] Implement persistent storage (PostgreSQL)
- [ ] Add Prometheus metrics
- [ ] Implement circuit breaker pattern
- [ ] Add authentication middleware (JWT)
- [ ] Create OpenAPI/Swagger documentation
- [ ] Add GraphQL support
- [ ] Implement event streaming (Kafka)
- [ ] Add distributed tracing (Jaeger)

---

## 📚 Additional Resources

- [NBCUniversal VPCR Platform Overview](https://www.nbcuniversal.com/technology)
- [Crossplane Provider Development](https://docs.crossplane.io/latest/concepts/providers/)
- [Go Context Package](https://pkg.go.dev/context)
- [Kubernetes Best Practices](https://kubernetes.io/docs/concepts/)

---

## 👤 Author

**Zhicheng Yu**  
Computer Science Student | Hunter College, CUNY  
📧 yuzhicheng369@gmail.com  
🔗 [LinkedIn](https://linkedin.com/in/zhichengyu369) | [GitHub](https://github.com/zhichengu1)

---

## 📄 License

This is an educational project built for interview preparation and learning purposes.

---

**Built with ❤️ to demonstrate production-grade engineering practices**
