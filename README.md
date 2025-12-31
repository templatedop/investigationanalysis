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
- **Multi-LLM Support**: Ollama, vLLM, OpenAI compatible providers
- **Human-in-the-Loop**: Escalation and review workflows
- **Comprehensive Audit Trail**: Full traceability of all decisions

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

## Usage

### API Endpoints

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
```

### CLI Usage

```bash
# Start investigation
./cli investigate --claim-id CLM001 --amount 50000 --type health

# Check status
./cli status --workflow-id investigation-CLM001

# Send decision signal
./cli signal --workflow-id investigation-CLM001 --decision approve

# Search knowledge base
./cli search --query "staged accident fraud patterns"
```

## Configuration

Configuration can be set via environment variables or `config.json`:

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
```

## RAG Knowledge Stores

The system uses five specialized vector stores:

1. **Policy Documents**: Insurance policy terms, exclusions, conditions
2. **Fraud Cases**: Historical fraud cases and patterns
3. **Entity Knowledge**: Known fraudsters, watchlists, relationships
4. **Playbooks**: Investigation procedures and guidelines
5. **External Knowledge**: Industry trends, regulatory updates

### Indexing Documents

```go
// Index a fraud case
manager.IndexFraudCase(ctx, &FraudCaseDocument{
    CaseID:        "FC001",
    FraudType:     "staged_accident",
    Description:   "Organized ring staging car accidents",
    ModusOperandi: "Multiple claimants, same garage, inflated estimates",
    Indicators:    []string{"multiple_claimants", "same_provider"},
    Resolution:    "Claims denied, referred to law enforcement",
})

// Index a policy
manager.IndexPolicy(ctx, &PolicyDocument{
    PolicyName:  "Health Insurance Gold Plan",
    PolicyType:  "health",
    Coverage:    "Hospitalization, surgery, prescriptions",
    Exclusions:  []string{"pre-existing conditions", "cosmetic procedures"},
})
```

## Project Structure

```
├── cmd/
│   ├── api/          # REST API server
│   ├── cli/          # Command-line interface
│   └── worker/       # Temporal worker
├── pkg/
│   ├── agents/       # Specialized agent implementations
│   ├── config/       # Configuration management
│   ├── embeddings/   # Embedding generation
│   ├── llm/          # LLM client implementations
│   ├── models/       # Domain models
│   ├── rag/          # RAG system core
│   ├── vectorstore/  # Vector database integration
│   └── workflows/    # Temporal workflows
├── migrations/       # Database migrations
├── docker-compose.yml
├── Dockerfile
└── README.md
```

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
