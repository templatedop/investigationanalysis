-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create customers table
CREATE TABLE IF NOT EXISTS customers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    risk_score DECIMAL(5,4) DEFAULT 0,
    account_open_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create customer addresses table
CREATE TABLE IF NOT EXISTS customer_addresses (
    id SERIAL PRIMARY KEY,
    customer_id TEXT REFERENCES customers(id),
    street TEXT,
    city TEXT,
    state TEXT,
    postal_code TEXT,
    country TEXT DEFAULT 'IN',
    is_primary BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create policies table
CREATE TABLE IF NOT EXISTS policies (
    policy_id TEXT PRIMARY KEY,
    customer_id TEXT REFERENCES customers(id),
    type TEXT NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    premium DECIMAL(12,2),
    coverage_amount DECIMAL(15,2),
    exclusions JSONB DEFAULT '[]',
    waiting_periods JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create claims table
CREATE TABLE IF NOT EXISTS claims (
    claim_id TEXT PRIMARY KEY,
    policy_id TEXT REFERENCES policies(policy_id),
    customer_id TEXT REFERENCES customers(id),
    type TEXT NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    description TEXT,
    status TEXT DEFAULT 'pending',
    is_fraud BOOLEAN DEFAULT false,
    submitted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    customer_id TEXT REFERENCES customers(id),
    type TEXT NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    currency TEXT DEFAULT 'INR',
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    source_account TEXT,
    target_account TEXT,
    location_lat DECIMAL(10,7),
    location_lng DECIMAL(10,7),
    location_city TEXT,
    location_country TEXT,
    device_id TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create investigations table
CREATE TABLE IF NOT EXISTS investigations (
    id UUID PRIMARY KEY,
    claim_id TEXT,
    customer_id TEXT,
    status TEXT DEFAULT 'pending',
    risk_score DECIMAL(5,4),
    risk_level TEXT,
    final_decision TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    audit_trail JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create vector stores for RAG

-- Policy documents store
CREATE TABLE IF NOT EXISTS vectors_policy_docs (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(768),
    metadata JSONB DEFAULT '{}',
    store_type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Fraud cases store
CREATE TABLE IF NOT EXISTS vectors_fraud_cases (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(768),
    metadata JSONB DEFAULT '{}',
    store_type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Entity knowledge store
CREATE TABLE IF NOT EXISTS vectors_entity_knowledge (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(768),
    metadata JSONB DEFAULT '{}',
    store_type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Playbooks store
CREATE TABLE IF NOT EXISTS vectors_playbooks (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(768),
    metadata JSONB DEFAULT '{}',
    store_type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- External knowledge store
CREATE TABLE IF NOT EXISTS vectors_external_knowledge (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding vector(768),
    metadata JSONB DEFAULT '{}',
    store_type TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes

CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);
CREATE INDEX IF NOT EXISTS idx_customers_phone ON customers(phone);

CREATE INDEX IF NOT EXISTS idx_policies_customer ON policies(customer_id);
CREATE INDEX IF NOT EXISTS idx_policies_type ON policies(type);
CREATE INDEX IF NOT EXISTS idx_policies_dates ON policies(start_date, end_date);

CREATE INDEX IF NOT EXISTS idx_claims_customer ON claims(customer_id);
CREATE INDEX IF NOT EXISTS idx_claims_policy ON claims(policy_id);
CREATE INDEX IF NOT EXISTS idx_claims_status ON claims(status);
CREATE INDEX IF NOT EXISTS idx_claims_fraud ON claims(is_fraud);

CREATE INDEX IF NOT EXISTS idx_transactions_customer ON transactions(customer_id);
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp);

CREATE INDEX IF NOT EXISTS idx_investigations_claim ON investigations(claim_id);
CREATE INDEX IF NOT EXISTS idx_investigations_status ON investigations(status);

-- Vector indexes (IVFFlat)
CREATE INDEX IF NOT EXISTS idx_policy_docs_embedding ON vectors_policy_docs
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_fraud_cases_embedding ON vectors_fraud_cases
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_entity_knowledge_embedding ON vectors_entity_knowledge
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_playbooks_embedding ON vectors_playbooks
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_external_knowledge_embedding ON vectors_external_knowledge
    USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Full-text search indexes
CREATE INDEX IF NOT EXISTS idx_policy_docs_content ON vectors_policy_docs
    USING GIN (to_tsvector('english', content));

CREATE INDEX IF NOT EXISTS idx_fraud_cases_content ON vectors_fraud_cases
    USING GIN (to_tsvector('english', content));

-- GIN indexes for JSONB metadata
CREATE INDEX IF NOT EXISTS idx_policy_docs_metadata ON vectors_policy_docs USING GIN (metadata);
CREATE INDEX IF NOT EXISTS idx_fraud_cases_metadata ON vectors_fraud_cases USING GIN (metadata);
CREATE INDEX IF NOT EXISTS idx_entity_knowledge_metadata ON vectors_entity_knowledge USING GIN (metadata);
