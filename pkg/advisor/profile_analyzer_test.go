package advisor

import (
	"context"
	"testing"
)

func TestProfileAnalyzer_Analyze_ValidUserID(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	profile, err := pa.Analyze(ctx, "user-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile == nil {
		t.Fatal("profile should not be nil")
	}
	if profile.UserID != "user-123" {
		t.Errorf("got user_id %s, want user-123", profile.UserID)
	}
	if profile.RiskTolerance != RiskModerate {
		t.Errorf("got risk %s, want moderate", profile.RiskTolerance)
	}
}

func TestProfileAnalyzer_Analyze_EmptyUserID(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	profile, err := pa.Analyze(ctx, "")

	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
	if profile != nil {
		t.Fatal("profile should be nil on error")
	}
}

func TestProfileAnalyzer_AnalyzeKYCStatus_HighValue(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	risk, err := pa.AnalyzeKYCStatus(ctx, "user-123", "verified_high_value")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk != RiskAggressive {
		t.Errorf("got risk %s, want aggressive for high_value", risk)
	}
}

func TestProfileAnalyzer_AnalyzeKYCStatus_Standard(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	risk, err := pa.AnalyzeKYCStatus(ctx, "user-123", "verified_standard")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk != RiskModerate {
		t.Errorf("got risk %s, want moderate for standard", risk)
	}
}

func TestProfileAnalyzer_AnalyzeKYCStatus_Pending(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	risk, err := pa.AnalyzeKYCStatus(ctx, "user-123", "pending")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risk != RiskConservative {
		t.Errorf("got risk %s, want conservative for pending", risk)
	}
}

func TestProfileAnalyzer_AnalyzeSpendingHistory(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	experience, err := pa.AnalyzeSpendingHistory(ctx, "user-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if experience != ExperienceBeginner {
		t.Errorf("got experience %s, want beginner", experience)
	}
}

func TestProfileAnalyzer_ExtractComplianceConstraints(t *testing.T) {
	pa := NewProfileAnalyzer()
	ctx := context.Background()

	constraints, err := pa.ExtractComplianceConstraints(ctx, "user-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if constraints == nil {
		t.Fatal("constraints should not be nil")
	}
	// Empty constraints is OK for unconstrained user
}

func TestProfileAnalyzer_ValidateProfile_Valid(t *testing.T) {
	pa := NewProfileAnalyzer()
	profile := &UserProfile{UserID: "user-123"}

	err := pa.ValidateProfile(profile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProfileAnalyzer_ValidateProfile_NilProfile(t *testing.T) {
	pa := NewProfileAnalyzer()

	err := pa.ValidateProfile(nil)

	if err == nil {
		t.Fatal("expected error for nil profile")
	}
}

func TestProfileAnalyzer_ValidateProfile_EmptyUserID(t *testing.T) {
	pa := NewProfileAnalyzer()
	profile := &UserProfile{UserID: ""}

	err := pa.ValidateProfile(profile)

	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}
