// Package commerce implements order → payment → settlement automation
// for the Genie multi-agent platform.
//
// Architecture Overview:
//
// Orders flow through a state machine: pending → paid → fulfilled,
// with rollback on failures. The workflow orchestrator coordinates:
//
//   1. Order creation & validation
//   2. Payment initiation (via Payment Agent)
//   3. Payment confirmation polling (via CBDC Bridge)
//   4. Settlement coordination (via Settlement Coordinator)
//   5. Order fulfillment
//
// On payment failure, the order transitions to payment_failed and
// awaits manual retry. On settlement failure, the order remains paid
// and the workflow is escalated to human-in-the-loop (HITL) review.
//
// Each step is logged to an audit trail for compliance and debugging.
//
// Components:
//
// - OrderManager: Manages order lifecycle (create, retrieve, update status)
// - WorkflowOrchestrator: Coordinates the multi-step order flow
// - CommerceAgent: Message-driven interface for agent bus integration
//
// Example Usage:
//
//   // Create dependencies
//   orderMgr := commerce.NewInMemoryOrderManager()
//   paymentAgent := paymentAgentImpl{}
//   settlementAgent := settlementAgentImpl{}
//   orchestrator := commerce.NewDefaultWorkflowOrchestrator(
//       orderMgr, paymentAgent, settlementAgent,
//   )
//
//   // Create an order
//   items := []commerce.OrderItem{
//       {SKU: "WIDGET", Description: "Widget", Quantity: 2, UnitPricePaise: 50_000},
//   }
//   order, err := orderMgr.CreateOrder("merchant-1", "customer-1", items)
//
//   // Execute the workflow (payment + settlement + fulfillment)
//   success, finalStatus, err := orchestrator.ExecuteWorkflow(ctx, order.ID)
//
//   // Retrieve final state
//   workflow, _ := orchestrator.GetWorkflow(order.ID)
//   auditLog, _ := orchestrator.GetAuditLog(order.ID)
//
// Thread Safety:
//
// InMemoryOrderManager and DefaultWorkflowOrchestrator are both
// thread-safe for concurrent order processing. Use them directly
// for testing; for production, implement persistent storage.
//
// Customization:
//
// Replace PaymentAgentStub and SettlementAgentStub interfaces with
// your own agent implementations that send messages via the Genie
// agent bus instead of making direct calls.
package commerce
