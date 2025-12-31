// Package main provides the REST API server for fraud investigation
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/workflows"
)

// Server holds the API server state
type Server struct {
	temporalClient client.Client
	httpServer     *http.Server
}

func main() {
	log.Println("Starting Fraud Investigation API Server...")

	// Connect to Temporal
	temporalClient, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}

	server := &Server{
		temporalClient: temporalClient,
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/api/v1/investigate", server.investigateHandler)
	mux.HandleFunc("/api/v1/investigation/", server.investigationHandler)
	mux.HandleFunc("/api/v1/investigations", server.listHandler)
	mux.HandleFunc("/api/v1/decision", server.decisionHandler)
	mux.HandleFunc("/api/v1/search", server.searchHandler)

	// Create HTTP server
	port := getEnv("PORT", "8080")
	server.httpServer = &http.Server{
		Addr:         ":" + port,
		Handler:      corsMiddleware(loggingMiddleware(mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("API Server listening on port %s", port)
		if err := server.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.httpServer.Shutdown(ctx)
	server.temporalClient.Close()
	log.Println("Server stopped")
}

// healthHandler returns server health status
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// InvestigateRequest is the request body for starting an investigation
type InvestigateRequest struct {
	ClaimID     string  `json:"claim_id"`
	PolicyID    string  `json:"policy_id"`
	CustomerID  string  `json:"customer_id"`
	ClaimType   string  `json:"claim_type"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Priority    int     `json:"priority"`
}

// investigateHandler starts a new investigation
func (s *Server) investigateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InvestigateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	input := &models.FraudAlertInput{
		ID:          req.ClaimID + "-alert",
		ClaimID:     req.ClaimID,
		PolicyID:    req.PolicyID,
		CustomerID:  req.CustomerID,
		ClaimType:   req.ClaimType,
		Amount:      req.Amount,
		Description: req.Description,
		SubmittedAt: time.Now(),
		Priority:    req.Priority,
	}

	workflowID := "investigation-" + req.ClaimID
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflows.TaskQueueName,
	}

	we, err := s.temporalClient.ExecuteWorkflow(r.Context(), options, workflows.FraudInvestigationWorkflow, input)
	if err != nil {
		http.Error(w, "Failed to start investigation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"workflow_id": we.GetID(),
		"run_id":      we.GetRunID(),
		"status":      "started",
	})
}

// investigationHandler gets investigation status or result
func (s *Server) investigationHandler(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Path[len("/api/v1/investigation/"):]
	if workflowID == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		resp, err := s.temporalClient.DescribeWorkflowExecution(r.Context(), workflowID, "")
		if err != nil {
			http.Error(w, "Investigation not found: "+err.Error(), http.StatusNotFound)
			return
		}

		result := map[string]interface{}{
			"workflow_id": workflowID,
			"status":      resp.WorkflowExecutionInfo.Status.String(),
			"start_time":  resp.WorkflowExecutionInfo.StartTime,
		}

		if resp.WorkflowExecutionInfo.CloseTime != nil {
			result["close_time"] = resp.WorkflowExecutionInfo.CloseTime
		}

		json.NewEncoder(w).Encode(result)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listHandler lists all investigations
func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := "WorkflowType = 'FraudInvestigationWorkflow'"
	resp, err := s.temporalClient.ListWorkflow(r.Context(), &client.ListWorkflowInput{
		Query: query,
	})
	if err != nil {
		http.Error(w, "Failed to list investigations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var investigations []map[string]interface{}
	for resp.HasNext() {
		execution := resp.Next()
		investigations = append(investigations, map[string]interface{}{
			"workflow_id": execution.Execution.WorkflowId,
			"run_id":      execution.Execution.RunId,
			"status":      execution.Status.String(),
			"start_time":  execution.StartTime,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"investigations": investigations,
		"count":          len(investigations),
	})
}

// DecisionRequest is the request for submitting a human decision
type DecisionRequest struct {
	WorkflowID string `json:"workflow_id"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
	ReviewerID string `json:"reviewer_id"`
}

// decisionHandler submits a human decision
func (s *Server) decisionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	decision := &models.HumanDecision{
		ReviewerID: req.ReviewerID,
		Action:     req.Action,
		Reason:     req.Reason,
		DecidedAt:  time.Now(),
	}

	err := s.temporalClient.SignalWorkflow(r.Context(), req.WorkflowID, "", workflows.HumanReviewSignal, decision)
	if err != nil {
		http.Error(w, "Failed to submit decision: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "decision_submitted",
		"message": "Human decision submitted successfully",
	})
}

// SearchRequest is the request for searching the knowledge base
type SearchRequest struct {
	Query      string   `json:"query"`
	StoreTypes []string `json:"store_types,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// searchHandler searches the knowledge base
func (s *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// This would use the RAG system to search
	// For now, return placeholder
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   req.Query,
		"results": []interface{}{},
		"message": "Search functionality requires RAG system initialization",
	})
}

// Middleware functions

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getEnv(name, defaultVal string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}
