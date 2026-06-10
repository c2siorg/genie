// Advisor Workspace Types
// Phase 7: Personalized financial recommendations
// Real API integration with backend pkg/advisor

// Action Types
export type ActionType =
  | 'reduce_category'
  | 'increase_savings'
  | 'move_funds'
  | 'invest'
  | 'refinance'
  | 'enroll_program';

export interface Action {
  id: string;
  type: ActionType;
  description: string;
  estimated_impact_paise: number;
  difficulty: 'easy' | 'medium' | 'hard';
  timeline_days: number;
  details?: Record<string, unknown>;
}

// Risk Types
export type RiskCategory = 'market' | 'liquidity' | 'counterparty' | 'regulatory';

export interface Risk {
  id: string;
  category: RiskCategory;
  description: string;
  severity: 'low' | 'medium' | 'high';
  mitigation: string;
}

// Recommendation Types
export type RecommendationCategory =
  | 'spending_optimization'
  | 'savings'
  | 'investment'
  | 'tax_efficiency'
  | 'risk_management';

export type RecommendationStatus =
  | 'pending'
  | 'accepted'
  | 'rejected'
  | 'deferred'
  | 'implemented';

export interface ImpactEstimate {
  monthly_savings_paise: number;
  annual_return_paise: number;
  confidence: number;
}

export interface Recommendation {
  recommendation_id: string;
  user_id: string;
  category: RecommendationCategory;
  title: string;
  description: string;
  rationale: string;
  actions: Action[];
  estimated_impact: ImpactEstimate;
  risks: Risk[];
  compliance_constraints: string[];
  status: RecommendationStatus;
  trace_id: string;
  created_at: string;
  updated_at: string;
}

// User Profile Types
export type RiskTolerance = 'conservative' | 'moderate' | 'aggressive';
export type InvestmentExperience = 'beginner' | 'intermediate' | 'expert';

export interface UserProfile {
  user_id: string;
  risk_tolerance: RiskTolerance;
  annual_income_paise: number;
  savings_goal_paise: number;
  investment_experience: InvestmentExperience;
  compliance_restrictions: string[];
  created_at: string;
  updated_at: string;
}

// Calibration Types
export interface CalibrationMetrics {
  user_id: string;
  recommendations_count: number;
  acceptance_rate: number;
  avg_impact_realized: number;
  accuracy_score: number;
  last_calibrated_at: string;
}

// Feedback Types
export type FeedbackAction = 'accept' | 'reject' | 'defer';

export interface RecommendationFeedback {
  recommendation_id: string;
  action: FeedbackAction;
  reason?: string;
  deferred_until?: string;
  feedback_at: string;
}

// API Request Types
export interface GenerateRecommendationRequest {
  user_id: string;
  category?: RecommendationCategory;
  context_document_id?: string;
  constraints?: {
    max_risk: number;
    min_return: number;
    timeline_months: number;
  };
}

export interface RecommendationHistoryRequest {
  user_id: string;
  limit?: number;
  status?: RecommendationStatus;
}

// API Response Types
export interface GenerateRecommendationResponse {
  recommendation_id: string;
  category: RecommendationCategory;
  title: string;
  description: string;
  actions: Action[];
  estimated_impact: ImpactEstimate;
  risks: Risk[];
  trace_id: string;
}

export interface RecommendationHistoryResponse {
  recommendations: Recommendation[];
  total_count: number;
  recommendations_accepted: number;
  recommendations_rejected: number;
  estimated_total_impact: {
    monthly_savings: number;
    annual_return: number;
  };
}

export interface CalibrationResponse extends CalibrationMetrics {}

// Stream Events
export type AdvisorStreamEvent =
  | { event: 'ai_disclosure'; data: string }
  | { event: 'trace'; data: { trace_id: string } }
  | { event: 'advisor.analyzing'; data: { step: string; progress: number } }
  | { event: 'advisor.generating'; data: { step: string; progress: number } }
  | { event: 'recommendation'; data: Recommendation }
  | { event: 'error'; data: { error: string } };
