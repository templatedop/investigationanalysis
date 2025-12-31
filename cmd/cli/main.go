// Package main provides the CLI for fraud investigation operations
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/fraudinvestigation/rag-framework/pkg/models"
	"github.com/fraudinvestigation/rag-framework/pkg/workflows"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "investigate":
		runInvestigation()
	case "status":
		getStatus()
	case "list":
		listInvestigations()
	case "signal":
		sendSignal()
	case "index":
		indexDocuments()
	case "search":
		searchKnowledge()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Fraud Investigation RAG CLI

Usage:
  cli <command> [arguments]

Commands:
  investigate    Start a new fraud investigation
  status         Get status of an investigation
  list           List all investigations
  signal         Send a human decision signal
  index          Index documents into knowledge store
  search         Search the knowledge store

Examples:
  cli investigate --claim-id CLM001 --amount 50000 --type health
  cli status --workflow-id <id>
  cli signal --workflow-id <id> --decision approve
  cli index --store fraud_cases --file cases.json
  cli search --query "staged accident patterns"
`)
}

func runInvestigation() {
	// Parse arguments
	claimID := getArg("--claim-id", "CLM-"+time.Now().Format("20060102150405"))
	amount := getArgFloat("--amount", 10000)
	claimType := getArg("--type", "health")
	description := getArg("--description", "Investigation triggered by alert")
	customerID := getArg("--customer-id", "CUST001")
	policyID := getArg("--policy-id", "POL001")

	input := &models.FraudAlertInput{
		ID:          claimID + "-alert",
		ClaimID:     claimID,
		PolicyID:    policyID,
		CustomerID:  customerID,
		ClaimType:   claimType,
		Amount:      amount,
		Description: description,
		SubmittedAt: time.Now(),
		Priority:    3,
	}

	// Connect to Temporal
	c, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	// Start workflow
	workflowID := "investigation-" + claimID
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: workflows.TaskQueueName,
	}

	we, err := c.ExecuteWorkflow(context.Background(), options, workflows.FraudInvestigationWorkflow, input)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Started investigation workflow\n")
	fmt.Printf("Workflow ID: %s\n", we.GetID())
	fmt.Printf("Run ID: %s\n", we.GetRunID())

	// Optionally wait for result
	if getArg("--wait", "") == "true" {
		var result *models.InvestigationResult
		if err := we.Get(context.Background(), &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}

		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("\nResult:\n%s\n", string(output))
	}
}

func getStatus() {
	workflowID := getArg("--workflow-id", "")
	if workflowID == "" {
		log.Fatal("--workflow-id is required")
	}

	c, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	resp, err := c.DescribeWorkflowExecution(context.Background(), workflowID, "")
	if err != nil {
		log.Fatalf("Failed to get workflow status: %v", err)
	}

	fmt.Printf("Workflow ID: %s\n", workflowID)
	fmt.Printf("Status: %s\n", resp.WorkflowExecutionInfo.Status)
	fmt.Printf("Start Time: %s\n", resp.WorkflowExecutionInfo.StartTime)
	if resp.WorkflowExecutionInfo.CloseTime != nil {
		fmt.Printf("Close Time: %s\n", resp.WorkflowExecutionInfo.CloseTime)
	}
}

func listInvestigations() {
	c, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	query := "WorkflowType = 'FraudInvestigationWorkflow'"
	resp, err := c.ListWorkflow(context.Background(), &client.ListWorkflowInput{
		Query: query,
	})
	if err != nil {
		log.Fatalf("Failed to list workflows: %v", err)
	}

	fmt.Printf("%-40s %-15s %-25s\n", "WORKFLOW ID", "STATUS", "START TIME")
	fmt.Println(string(make([]byte, 80)))

	for resp.HasNext() {
		execution := resp.Next()
		fmt.Printf("%-40s %-15s %-25s\n",
			execution.Execution.WorkflowId,
			execution.Status,
			execution.StartTime.Format(time.RFC3339),
		)
	}
}

func sendSignal() {
	workflowID := getArg("--workflow-id", "")
	decision := getArg("--decision", "")

	if workflowID == "" || decision == "" {
		log.Fatal("--workflow-id and --decision are required")
	}

	c, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer c.Close()

	humanDecision := &models.HumanDecision{
		ReviewerID: getEnv("USER", "cli-user"),
		Action:     decision,
		Reason:     getArg("--reason", "Decision made via CLI"),
		DecidedAt:  time.Now(),
	}

	err = c.SignalWorkflow(context.Background(), workflowID, "", workflows.HumanReviewSignal, humanDecision)
	if err != nil {
		log.Fatalf("Failed to send signal: %v", err)
	}

	fmt.Printf("Sent %s decision to workflow %s\n", decision, workflowID)
}

func indexDocuments() {
	// Placeholder for indexing documents
	store := getArg("--store", "")
	file := getArg("--file", "")

	if store == "" || file == "" {
		log.Fatal("--store and --file are required")
	}

	fmt.Printf("Indexing documents from %s into %s store...\n", file, store)
	fmt.Println("(This requires the worker to be running with RAG system)")
}

func searchKnowledge() {
	query := getArg("--query", "")
	if query == "" {
		log.Fatal("--query is required")
	}

	fmt.Printf("Searching for: %s\n", query)
	fmt.Println("(This requires the worker to be running with RAG system)")
}

// Helper functions

func getArg(name, defaultVal string) string {
	for i, arg := range os.Args {
		if arg == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return defaultVal
}

func getArgFloat(name string, defaultVal float64) float64 {
	val := getArg(name, "")
	if val == "" {
		return defaultVal
	}
	var f float64
	fmt.Sscanf(val, "%f", &f)
	return f
}

func getEnv(name, defaultVal string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}
