package consent

import (
	"fmt"
	"time"
)

// ExampleConsentMatrix demonstrates basic grant/check/revoke operations.
func ExampleConsentMatrix() {
	// Create a consent matrix and access log
	matrix := NewInMemoryConsentMatrix()

	// Agent-admin grants Agent-b read-only access to the database for 24 hours
	grantID, _ := matrix.Grant(
		"agent-b",           // userID
		"database",          // resourceType
		PermRead,            // permissionLevel
		24*time.Hour,        // ttl
		"data analysis job", // reason
		"agent-admin",       // grantedBy
	)
	fmt.Println("Granted consent:", grantID != "")

	// Check if agent-b can read from database
	allowed, _, _ := matrix.Check("agent-b", "database", ActionRead)
	fmt.Println("agent-b can read database:", allowed)

	// Check if agent-b can write to database (should fail)
	allowed, _, _ = matrix.Check("agent-b", "database", ActionWrite)
	fmt.Println("agent-b can write database:", allowed)

	// List all grants for agent-b
	grants, _ := matrix.List("agent-b")
	fmt.Printf("agent-b has %d grants\n", len(grants))

	// Revoke the consent
	_ = matrix.Revoke("agent-b", "database")
	allowed, _, _ = matrix.Check("agent-b", "database", ActionRead)
	fmt.Println("after revoke, agent-b can read database:", allowed)

	// Output:
	// Granted consent: true
	// agent-b can read database: true
	// agent-b can write database: false
	// agent-b has 1 grants
	// after revoke, agent-b can read database: false
}

// ExampleConsentChecker demonstrates the integrated checker with audit logging.
func ExampleConsentChecker() {
	// Create components
	matrix := NewInMemoryConsentMatrix()
	log := NewInMemoryAccessLog()
	checker := NewConsentChecker(matrix, log)

	// Grant permission
	matrix.Grant("agent-processor", "data-lake", PermReadWrite, 1*time.Hour, "ETL job", "admin")

	// Check and log an allowed action
	allowed, reasonCode, _ := checker("agent-processor", "data-lake", ActionWrite, "trace-001")
	fmt.Printf("Access decision: allowed=%v, reason=%q\n", allowed, reasonCode)

	// Check and log a denied action (no grant)
	allowed, reasonCode, _ = checker("agent-unknown", "data-lake", ActionRead, "trace-002")
	fmt.Printf("Access decision: allowed=%v, reason=%q\n", allowed, reasonCode)

	// Query the log
	decisions, _ := log.Query("agent-processor", time.Time{}, time.Time{})
	fmt.Printf("Decisions logged for agent-processor: %d\n", len(decisions))

	// Export to CSV for audit
	csvData, _ := log.Export("csv")
	fmt.Printf("Exported log as CSV: %d bytes\n", len(csvData))

	// Output:
	// Access decision: allowed=true, reason=""
	// Access decision: allowed=false, reason="no_grant"
	// Decisions logged for agent-processor: 1
	// Exported log as CSV: 215 bytes
}

// ExampleConsentRecord demonstrates the permission hierarchy.
func ExampleConsentRecord() {
	// Create records with different permission levels
	read := ConsentRecord{Permissions: PermRead}
	readWrite := ConsentRecord{Permissions: PermReadWrite}
	admin := ConsentRecord{Permissions: PermAdmin}

	// Test what each permission allows
	fmt.Println("PermRead allows:", []string{
		"read:" + fmt.Sprintf("%v", read.AllowsAction(ActionRead)),
		"write:" + fmt.Sprintf("%v", read.AllowsAction(ActionWrite)),
	})

	fmt.Println("PermReadWrite allows:", []string{
		"read:" + fmt.Sprintf("%v", readWrite.AllowsAction(ActionRead)),
		"write:" + fmt.Sprintf("%v", readWrite.AllowsAction(ActionWrite)),
		"delete:" + fmt.Sprintf("%v", readWrite.AllowsAction(ActionDelete)),
	})

	fmt.Println("PermAdmin allows:", []string{
		"read:" + fmt.Sprintf("%v", admin.AllowsAction(ActionRead)),
		"write:" + fmt.Sprintf("%v", admin.AllowsAction(ActionWrite)),
		"delete:" + fmt.Sprintf("%v", admin.AllowsAction(ActionDelete)),
		"admin:" + fmt.Sprintf("%v", admin.AllowsAction(ActionAdmin)),
	})

	// Output:
	// PermRead allows: [read:true write:false]
	// PermReadWrite allows: [read:true write:true delete:false]
	// PermAdmin allows: [read:true write:true delete:true admin:true]
}

// ExampleInMemoryAccessLog demonstrates log statistics and queries.
func ExampleInMemoryAccessLog() {
	log := NewInMemoryAccessLog()
	now := time.Now().UTC()

	// Simulate some authorization decisions
	for i := 0; i < 5; i++ {
		log.Log(AccessDecision{
			Timestamp:    now.Add(time.Duration(i) * time.Minute),
			UserID:       fmt.Sprintf("agent-%d", i%2),
			ResourceType: "database",
			Action:       ActionRead,
			Result:       "allowed",
			ReasonCode:   "",
		})
	}

	// Add some denials
	for i := 0; i < 3; i++ {
		log.Log(AccessDecision{
			Timestamp:    now.Add(time.Duration(10+i) * time.Minute),
			UserID:       "unknown-agent",
			ResourceType: "api",
			Action:       ActionWrite,
			Result:       "denied",
			ReasonCode:   "no_grant",
		})
	}

	// Get statistics
	stats := log.GetStats()
	fmt.Printf("Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("Allowed: %d, Denied: %d\n", stats.AllowedCount, stats.DeniedCount)
	fmt.Printf("Unique users: %d, Unique resources: %d\n", stats.UniqueUsers, stats.UniqueResources)

	// Query specific user
	decisions, _ := log.Query("agent-0", time.Time{}, time.Time{})
	fmt.Printf("Decisions for agent-0: %d\n", len(decisions))

	// Output:
	// Total entries: 8
	// Allowed: 5, Denied: 3
	// Unique users: 3, Unique resources: 2
	// Decisions for agent-0: 3
}
