package contractv1

import "testing"

func TestProfileAnalyzerInput_Validate(t *testing.T) {
	if err := (&ProfileAnalyzerInput{UserID: "u1"}).Validate(); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
	if err := (&ProfileAnalyzerInput{}).Validate(); err == nil {
		t.Error("missing user_id should fail")
	}
	if err := (&ProfileAnalyzerInput{UserID: "u1", AnnualIncomePaise: -1}).Validate(); err == nil {
		t.Error("negative income should fail")
	}
	var nilIn *ProfileAnalyzerInput
	if err := nilIn.Validate(); err == nil {
		t.Error("nil input should fail")
	}
}

func TestFinancialAnalystInput_Validate(t *testing.T) {
	ok := &FinancialAnalystInput{Profile: UserProfile{UserID: "u1"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
	if err := (&FinancialAnalystInput{}).Validate(); err == nil {
		t.Error("missing profile.user_id should fail")
	}
}

func TestRecommendationGeneratorInput_Validate(t *testing.T) {
	ok := &RecommendationGeneratorInput{
		Profile:       UserProfile{UserID: "u1"},
		Opportunities: []SpendingOpportunity{{ID: "o1"}},
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
	if err := (&RecommendationGeneratorInput{Profile: UserProfile{UserID: "u1"}}).Validate(); err == nil {
		t.Error("no opportunities should fail")
	}
	if err := (&RecommendationGeneratorInput{Opportunities: []SpendingOpportunity{{ID: "o1"}}}).Validate(); err == nil {
		t.Error("missing profile.user_id should fail")
	}
}

func TestRecommendation_Validate(t *testing.T) {
	good := &Recommendation{
		RecommendationID: "r1", UserID: "u1", Title: "t",
		EstimatedImpact: ImpactEstimate{Confidence: 0.5},
	}
	if err := good.Validate(); err != nil {
		t.Errorf("valid recommendation rejected: %v", err)
	}
	for name, r := range map[string]*Recommendation{
		"nil":            nil,
		"no id":          {UserID: "u1", Title: "t"},
		"no user":        {RecommendationID: "r1", Title: "t"},
		"no title":       {RecommendationID: "r1", UserID: "u1"},
		"bad confidence": {RecommendationID: "r1", UserID: "u1", Title: "t", EstimatedImpact: ImpactEstimate{Confidence: 1.5}},
	} {
		if err := r.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}
