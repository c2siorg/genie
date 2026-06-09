package judges

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// LineageJudge evaluates audit trail integrity for:
//   - HashChainIntegrity: lineage entries immutable and linked by hash chain
//   - DecisionTraceability: audit provides full decision context
//   - TimestampMonotonicity: timestamps strictly increasing within order
//   - CompleteLogs: all required audit fields present
type LineageJudge struct {
	cfg        JudgeConfig
	rubricID   string
	rubricName string
	calibration *CalibrationResult
}

// NewLineageJudge creates a new lineage judge.
func NewLineageJudge(cfg JudgeConfig) *LineageJudge {
	return &LineageJudge{
		cfg:        cfg,
		rubricID:   "RB-LA-001", // Audit Trail Immutability and Hash Chain (primary)
		rubricName: "Lineage Evaluation",
	}
}

// Name returns the judge's human-readable name.
func (lj *LineageJudge) Name() string {
	return "LineageJudge"
}

// Evaluate assesses lineage integrity.
func (lj *LineageJudge) Evaluate(ctx context.Context, input LineageJudgeInput) (Verdict, error) {
	now := time.Now()

	// Perform deterministic checks first
	hashChainValid := verifyHashChain(input.LineageEntries)
	timestampsMonotonic := verifyTimestampMonotonicity(input.LineageEntries)
	requiredFieldsPresent := verifyRequiredFields(input.LineageEntries)

	// If deterministic checks fail, verdict is fail
	if !hashChainValid || !timestampsMonotonic || !requiredFieldsPresent {
		return Verdict{
			Pass:        false,
			Score:       0.0,
			RubricID:    lj.rubricID,
			RubricName:  lj.rubricName,
			Reason:      "Deterministic check failed: hash chain invalid or timestamps non-monotonic",
			Evidence:    fmt.Sprintf("hashChainValid=%v, timestampsMonotonic=%v, requiredFieldsPresent=%v", hashChainValid, timestampsMonotonic, requiredFieldsPresent),
			EvaluatedAt: now,
		}, nil
	}

	// Build evaluation prompt for LLM judge
	systemPrompt := lineageJudgeSystemPrompt()
	userPrompt := lineageJudgeUserPrompt(input)

	// Call LLM judge for final assessment
	judgeResp, err := callLLMJudge(ctx, lj.cfg, systemPrompt, userPrompt)
	if err != nil {
		return Verdict{}, fmt.Errorf("callLLMJudge: %w", err)
	}

	// Apply calibration threshold if available
	score := judgeResp.Score
	if lj.calibration != nil {
		judgeResp.Pass = score >= lj.calibration.OptimalThreshold
	}

	return Verdict{
		Pass:        judgeResp.Pass,
		Score:       score,
		RubricID:    lj.rubricID,
		RubricName:  lj.rubricName,
		Reason:      judgeResp.Reason,
		Evidence:    judgeResp.Evidence,
		EvaluatedAt: now,
	}, nil
}

// Calibrate runs calibration on ground truth samples.
func (lj *LineageJudge) Calibrate(ctx context.Context, samples []LineageJudgeInput, groundTruth []bool) (CalibrationResult, error) {
	if len(samples) != len(groundTruth) {
		return CalibrationResult{}, fmt.Errorf("samples and groundTruth length mismatch")
	}

	if len(samples) == 0 {
		return CalibrationResult{}, fmt.Errorf("no samples provided")
	}

	result := CalibrationResult{
		CalibratedAt: time.Now(),
	}

	// Evaluate each sample
	var verdicts []bool
	var scores []float64
	calibrationSamples := make([]CalibrationSample, len(samples))

	for i, sample := range samples {
		verdict, err := lj.Evaluate(ctx, sample)
		if err != nil {
			return result, fmt.Errorf("evaluate sample %d: %w", i, err)
		}

		verdicts = append(verdicts, verdict.Pass)
		scores = append(scores, verdict.Score)
		calibrationSamples[i] = CalibrationSample{
			ID:          fmt.Sprintf("lineage-%d", i),
			GroundTruth: groundTruth[i],
			Verdict:     verdict.Pass,
			Score:       verdict.Score,
			Reason:      verdict.Reason,
		}
	}

	// Compute TPR and TNR
	result.TPR = computeTPR(verdicts, groundTruth)
	result.TNR = computeTNR(verdicts, groundTruth)

	// Compute 95% confidence intervals
	tprCI := bootstrapCI(scores, 1000)
	result.TPRLowerBound = tprCI.LowerBound
	result.TPRUpperBound = tprCI.UpperBound

	tnrCI := bootstrapCI(scores, 1000)
	result.TNRLowerBound = tnrCI.LowerBound
	result.TNRUpperBound = tnrCI.UpperBound

	// Find optimal threshold
	optimalThreshold, _ := findOptimalThreshold(scores, groundTruth)
	result.OptimalThreshold = optimalThreshold

	// Count positive/negative in ground truth
	for _, gt := range groundTruth {
		if gt {
			result.NumPositives++
		} else {
			result.NumNegatives++
		}
	}

	result.Samples = calibrationSamples

	// Update internal calibration
	lj.calibration = &result

	return result, nil
}

// verifyHashChain performs deterministic hash chain verification.
func verifyHashChain(entries []LineageEntry) bool {
	if len(entries) == 0 {
		return true // Empty chain is valid
	}

	for i := 1; i < len(entries); i++ {
		prev := entries[i-1]
		curr := entries[i]

		// Verify previous hash link
		if curr.PreviousHash != prev.Hash {
			return false
		}

		// Verify current entry hash (compute expected hash)
		expectedHash := computeEntryHash(curr.OrderID, curr.Step, curr.Timestamp.String(), curr.Data)
		if curr.Hash != expectedHash {
			return false
		}
	}

	return true
}

// verifyTimestampMonotonicity checks that timestamps are strictly increasing.
func verifyTimestampMonotonicity(entries []LineageEntry) bool {
	if len(entries) <= 1 {
		return true // 0 or 1 entries is trivially monotonic
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.Before(entries[i-1].Timestamp) ||
			entries[i].Timestamp.Equal(entries[i-1].Timestamp) {
			return false // Not strictly increasing
		}
	}

	return true
}

// verifyRequiredFields checks that all entries have required fields.
func verifyRequiredFields(entries []LineageEntry) bool {
	for _, entry := range entries {
		if entry.EntryID == "" || entry.OrderID == "" || entry.Step == "" ||
			entry.Data == "" || entry.Hash == "" {
			return false
		}
	}
	return true
}

// computeEntryHash computes the SHA256 hash of a lineage entry.
func computeEntryHash(orderID, step, timestamp, data string) string {
	h := sha256.New()
	h.Write([]byte(orderID + "|" + step + "|" + timestamp + "|" + data))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// lineageJudgeSystemPrompt returns the system prompt for lineage judge.
func lineageJudgeSystemPrompt() string {
	return `You are a Lineage/Audit Trail Evaluation Judge for a regulatory-compliant financial system.

Your job is to assess whether the audit trail is complete, immutable, and provides full decision traceability.

Respond ONLY with a JSON object in this exact format (no markdown, no extra text):
{"pass": <boolean>, "score": <float 0.0-1.0>, "reason": "<brief explanation>", "evidence": "<observed facts>"}

Scoring rubric:
  pass=true (score ≥0.8): Hash chain valid, timestamps monotonic, all required fields present, full decision context
  pass=true (score 0.6-0.8): Hash chain valid, timestamps monotonic, mostly complete fields
  pass=false (score 0.4-0.6): Hash chain issues, timestamp gaps, missing fields
  pass=false (score <0.4): Hash chain broken, timestamps non-monotonic, critical fields missing

Key evaluation points:
1. HashChainIntegrity: Are entries linked by hash chain (each entry commits previous)?
2. TimestampMonotonicity: Are timestamps strictly increasing within the order?
3. CompleteLogs: Are all required fields present (entryID, orderID, step, timestamp, data, hash)?
4. DecisionTraceability: Does audit trail provide full context for understanding decisions made?

Be precise and reference specific entries from the provided context.`
}

// lineageJudgeUserPrompt formats the user prompt with lineage details.
func lineageJudgeUserPrompt(input LineageJudgeInput) string {
	entriesStr := "["
	for i, entry := range input.LineageEntries {
		if i > 0 {
			entriesStr += ", "
		}
		entriesStr += fmt.Sprintf("{entryID: %s, step: %s, timestamp: %s, hash: %s...}",
			entry.EntryID, entry.Step, entry.Timestamp.Format(time.RFC3339), entry.Hash[:8])
	}
	entriesStr += "]"

	return fmt.Sprintf(`Evaluate lineage integrity for this order:

Order Details:
- OrderID: %s

Lineage Entries:
%s
(Total: %d entries)

Lineage Properties:
- Hash Chain Valid: %v
- Timestamps Monotonic: %v
- All Required Fields Present: %v

Decision Traceability Context:
%s

Additional Context:
%v

Lineage Questions:
1. HashChainIntegrity: Each entry commits hash of previous (immutable)?
2. TimestampMonotonicity: Timestamps strictly increasing?
3. CompleteLogs: Required fields (entryID, orderID, step, timestamp, data, hash) all present?
4. DecisionTraceability: Audit provides full context for decisions made?

Assess whether this audit trail is complete, immutable, and provides full decision traceability.`,
		input.OrderID,
		entriesStr,
		len(input.LineageEntries),
		input.HashChainValid,
		input.TimestampsMonotonic,
		input.AllRequiredFieldsPresent,
		input.DecisionTraceability,
		input.Metadata,
	)
}
