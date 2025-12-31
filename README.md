# Fraud Investigation RAG Framework

A comprehensive multi-agent Retrieval-Augmented Generation (RAG) framework for fraud investigation, built with Go and Temporal workflows.

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                    MULTI-AGENT FRAUD INVESTIGATION ARCHITECTURE                      │
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────┐    │
│  │                         ORCHESTRATOR AGENT                                   │    │
│  │  • Coordinates all agents                                                    │    │
│  │  • Maintains investigation state                                             │    │
│  │  • Routes to appropriate specialists                                         │    │
│  │  • Manages human-in-the-loop escalation                                      │    │
│  └─────────────────────────────────────────────────────────────────────────────┘    │
│                                       │                                              │
│       ┌───────────────┬───────────────┼───────────────┬───────────────┐             │
│       ▼               ▼               ▼               ▼               ▼             │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐           │
│  │ DATA    │    │ ENTITY  │    │ PATTERN │    │ DOCUMENT│    │ POLICY  │           │
│  │ ENRICHER│    │ RESOLVER│    │ ANALYST │    │ ANALYST │    │ CHECKER │           │
│  └─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘           │
│       │               │               │               │               │             │
│       ▼               ▼               ▼               ▼               ▼             │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐           │
│  │ NETWORK │    │ TEMPORAL│    │ RISK    │    │ EVIDENCE│    │ EXPLAIN │           │
│  │ ANALYST │    │ ANALYST │    │ ASSESSOR│    │ COMPILER│    │ AGENT   │           │
│  └─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘           │
│                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────┐    │
│  │                         RAG KNOWLEDGE STORES                                 │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │    │
│  │  │ Policy Docs  │  │ Fraud Cases  │  │ Entity       │  │ Playbooks    │     │    │
│  │  │ (pgvector)   │  │ History      │  │ Knowledge    │  │              │     │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │    │
│  └─────────────────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

## Features

- **11 Specialized Agents**: Each agent handles a specific aspect of fraud investigation
- **RAG with pgvector**: Vector similarity search for relevant knowledge retrieval
- **Temporal Workflows**: Durable, reliable workflow orchestration
- **LangGraph-Style Execution**: Optional graph-based agent orchestration (configurable)
- **Multi-LLM Support**: Ollama, vLLM, OpenAI compatible providers
- **Human-in-the-Loop**: Escalation and review workflows
- **Comprehensive Audit Trail**: Full traceability of all decisions
- **Observability**: Structured logging, Prometheus metrics, health checks

## Agents

| Agent | Function | Model Type |
|-------|----------|------------|
| **Orchestrator** | Coordinates workflow | Fast (7B) |
| **Data Enricher** | Gathers context | Fast (7B) |
| **Entity Resolver** | Links identities | Medium (14B) |
| **Pattern Analyst** | Detects anomalies | Reasoning (32B) |
| **Network Analyst** | Maps relationships | Reasoning (32B) |
| **Temporal Analyst** | Timeline analysis | Medium (14B) |
| **Document Analyst** | Verifies documents | Vision |
| **Policy Checker** | Compliance checks | Medium (14B) |
| **Risk Assessor** | Final scoring | Reasoning (32B) |
| **Evidence Compiler** | Builds case | Medium (14B) |
| **Explainer** | Human communication | Medium (14B) |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+
- GPU with CUDA support (optional, for local LLM)

### Start with Docker

```bash
# Start all services
docker-compose up -d

# Pull required LLM models
docker exec -it investigationanalysis-ollama-1 ollama pull nomic-embed-text
docker exec -it investigationanalysis-ollama-1 ollama pull deepseek-r1:32b

# Check services
docker-compose ps
```

### Start with Monitoring (Prometheus + Grafana)

```bash
# Start all services including monitoring
docker-compose --profile monitoring up -d

# Access services:
# - API: http://localhost:8080
# - Temporal UI: http://localhost:8088
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

### Manual Setup

```bash
# Install dependencies
go mod download

# Run database migrations
psql -h localhost -U postgres -d fraud_investigation -f migrations/001_init.sql

# Start worker
go run cmd/worker/main.go

# Start API (in another terminal)
go run cmd/api/main.go
```

## Configuration

### Environment Variables

```bash
# Database
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=fraud_investigation

# Temporal
export TEMPORAL_HOST=localhost:7233
export TEMPORAL_NAMESPACE=default

# LLM
export LLM_PROVIDER=ollama
export LLM_BASE_URL=http://localhost:11434
export LLM_MODEL=deepseek-r1:32b

# Embeddings
export EMBEDDING_PROVIDER=ollama
export EMBEDDING_BASE_URL=http://localhost:11434
export EMBEDDING_MODEL=nomic-embed-text

# LangGraph Mode (enable graph-based execution)
export GRAPH_ENABLED=false

# Observability
export LOG_LEVEL=info       # debug, info, warn, error, fatal
export LOG_FORMAT=json      # json, text
export METRICS_ENABLED=true
export HEALTH_ENABLED=true
export TRACING_ENABLED=false
export TRACING_ENDPOINT=localhost:4317
```

### Configuration File (config.json)

```json
{
  "server": {
    "port": 8080,
    "enable_cors": true
  },
  "graph": {
    "enabled": false,
    "max_iterations": 100,
    "enable_checkpoints": true,
    "detect_loops": true
  },
  "observability": {
    "logging": {
      "enabled": true,
      "level": "info",
      "format": "json"
    },
    "metrics": {
      "enabled": true,
      "endpoint": "/metrics",
      "namespace": "fraud_investigation"
    },
    "health": {
      "enabled": true,
      "liveness_path": "/health/live",
      "readiness_path": "/health/ready"
    }
  }
}
```

## LangGraph-Style Execution

The framework supports two execution modes:

### 1. Temporal Workflow Mode (Default)

Traditional Temporal-based workflow execution with:
- Durable execution
- Automatic retries
- Activity heartbeats
- Workflow versioning

### 2. Graph Mode (Optional)

LangGraph-style state graph execution with:
- Conditional edge routing
- Parallel node execution
- Checkpoints for resumption
- Human-in-the-loop interrupts
- Streaming state updates

**Enable Graph Mode:**

```bash
export GRAPH_ENABLED=true
```

Or in config.json:
```json
{
  "graph": {
    "enabled": true
  }
}
```

## Observability

### Health Checks

```bash
# Liveness check (is the service alive?)
curl http://localhost:8080/health/live

# Readiness check (is the service ready for traffic?)
curl http://localhost:8080/health/ready
```

### Metrics (Prometheus)

Available metrics at `/metrics`:

- `fraud_investigation_investigations_total` - Total investigations by status
- `fraud_investigation_investigations_active` - Currently active investigations
- `fraud_investigation_investigation_duration_seconds` - Investigation duration histogram
- `fraud_investigation_risk_scores` - Distribution of risk scores
- `fraud_investigation_decisions_total` - Total decisions by type
- `fraud_investigation_escalations_total` - Escalations by reason
- `fraud_investigation_agent_executions_total` - Agent executions by agent and status
- `fraud_investigation_agent_duration_seconds` - Agent execution duration
- `fraud_investigation_rag_queries_total` - RAG queries by store
- `fraud_investigation_rag_latency_seconds` - RAG query latency

### Logging

Structured JSON logs with configurable levels:

```json
{
  "level": "INFO",
  "message": "Investigation completed",
  "timestamp": "2024-01-15T10:30:00Z",
  "fields": {
    "investigation_id": "inv-123",
    "claim_id": "CLM001",
    "decision": "APPROVED",
    "risk_score": 0.25
  }
}
```

## API Endpoints

```bash
# Start an investigation
curl -X POST http://localhost:8080/api/v1/investigate \
  -H "Content-Type: application/json" \
  -d '{
    "claim_id": "CLM001",
    "policy_id": "POL001",
    "customer_id": "CUST001",
    "claim_type": "health",
    "amount": 50000,
    "description": "Medical claim for surgery",
    "priority": 3
  }'

# Check status
curl http://localhost:8080/api/v1/investigation/investigation-CLM001

# Submit human decision
curl -X POST http://localhost:8080/api/v1/decision \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_id": "investigation-CLM001",
    "action": "APPROVE",
    "reason": "Verified documentation",
    "reviewer_id": "reviewer@example.com"
  }'

# List all investigations
curl http://localhost:8080/api/v1/investigations

# Health and metrics
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
curl http://localhost:8080/metrics
```

## Project Structure

```
├── cmd/
│   ├── api/              # REST API server
│   ├── cli/              # Command-line interface
│   └── worker/           # Temporal worker
├── pkg/
│   ├── agents/           # Specialized agent implementations
│   ├── config/           # Configuration management
│   ├── embeddings/       # Embedding generation
│   ├── graph/            # LangGraph-style execution
│   │   ├── state.go      # State management
│   │   ├── node.go       # Node types
│   │   ├── edge.go       # Edge routing
│   │   ├── graph.go      # Graph builder
│   │   └── executor.go   # Graph executor
│   ├── llm/              # LLM client implementations
│   ├── models/           # Domain models
│   ├── observability/    # Observability components
│   │   ├── logging/      # Structured logging
│   │   ├── metrics/      # Prometheus metrics
│   │   └── health/       # Health checks
│   ├── rag/              # RAG system core
│   ├── vectorstore/      # Vector database integration
│   └── workflows/        # Temporal workflows
├── tests/
│   ├── unit/             # Unit tests
│   └── integration/      # Integration tests
├── migrations/           # Database migrations
├── docker-compose.yml
├── prometheus.yml        # Prometheus configuration
├── Dockerfile
└── README.md
```

## Testing

```bash
# Run unit tests
go test ./tests/unit/... -v

# Run integration tests
go test ./tests/integration/... -v

# Run all tests with coverage
go test ./... -cover

# Skip long-running tests
go test ./... -short
```

## RAG Knowledge Stores

The system uses five specialized vector stores:

1. **Policy Documents**: Insurance policy terms, exclusions, conditions
2. **Fraud Cases**: Historical fraud cases and patterns
3. **Entity Knowledge**: Known fraudsters, watchlists, relationships
4. **Playbooks**: Investigation procedures and guidelines
5. **External Knowledge**: Industry trends, regulatory updates

## Workflow Phases

1. **Data Gathering** (Parallel)
   - Data Enrichment Agent
   - Entity Resolution Agent

2. **Analysis** (Parallel)
   - Pattern Analysis Agent
   - Network Analysis Agent
   - Temporal Analysis Agent
   - Document Analysis Agent
   - Policy Compliance Agent

3. **Risk Assessment** (Sequential)
   - Risk Assessment Agent

4. **Decision & Escalation**
   - Human review if needed
   - Automatic escalation on timeout

5. **Evidence Compilation**
   - Evidence Compilation Agent
   - Report generation

6. **Action Execution**
   - Execute decision
   - Notifications

## Temporal Web UI

Access the Temporal UI at http://localhost:8088 to:
- Monitor running workflows
- View workflow history
- Debug failed executions
- Query workflow state

## License

MIT License
