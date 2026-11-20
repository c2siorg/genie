# Phase 8: Analytics Workspace Specification

**Status**: 📋 Planned  
**Planned APIs**: 8 endpoints  
**Planned Types**: 10 type definitions  
**Planned Tests**: 18 tests  

---

## Specification

### Overview

The Analytics workspace provides **financial insights and performance dashboards** for users. While the Assistant answers questions and the Advisor provides recommendations, Analytics shows trends, patterns, and metrics across transaction history, compliance status, and financial goals.

**Core Value**:
- Spend trends (daily/weekly/monthly)
- Category analysis (pie charts, top categories)
- Savings progress (vs. goals)
- Investment performance (if applicable)
- Compliance metrics (velocity usage, AML flags)
- Recommendation impact tracking (actual vs estimated savings)

---

### API Endpoints

#### 1. Transaction Summary
```
GET /v1/analytics/summary?period=day|week|month|year
Response:
  - period: string
  - income_paise: number
  - expense_paise: number
  - net_paise: number
  - category_breakdown: {category: string, amount_paise: number}[]
  - top_spending_categories: {category: string, percentage: number}[]
  - timestamp: RFC3339
```

#### 2. Spending Trends
```
GET /v1/analytics/trends?category=string&days=30
Response:
  - category?: string
  - datapoints: {date: RFC3339, amount_paise: number}[]
  - trend_direction: "up" | "down" | "stable"
  - avg_daily_paise: number
  - change_percent: number (vs previous period)
```

#### 3. Budget Progress
```
GET /v1/analytics/budget/{category}?period=month
Response:
  - category: string
  - budget_paise: number
  - spent_paise: number
  - remaining_paise: number
  - percentage_used: number
  - status: "on_track" | "warning" | "exceeded"
  - projected_end_of_period_paise: number
```

#### 4. Savings Goals
```
GET /v1/analytics/goals
Response:
  - goals: SavingsGoal[]
    - goal_id, name, target_paise, current_paise
    - deadline: RFC3339, progress_percent
    - on_track: boolean
```

#### 5. Savings Progress
```
GET /v1/analytics/savings-progress?goal_id=string&days=90
Response:
  - goal_id, goal_name, target_paise
  - datapoints: {date: RFC3339, saved_paise: number}[]
  - projection_on_track: boolean
  - projected_completion: RFC3339
```

#### 6. Recommendation Impact
```
GET /v1/analytics/recommendation-impact?recommendation_id=string
Response:
  - recommendation_id
  - estimated_impact_paise: number
  - actual_impact_paise: number
  - accuracy_percent: number (actual / estimated * 100)
  - implemented_date: RFC3339
  - impact_datapoints: {date: RFC3339, cumulative_paise: number}[]
```

#### 7. Peer Benchmarking
```
GET /v1/analytics/benchmarks?segment=age_range|income_bracket|lifestyle
Response:
  - segment: string
  - user_metrics: {metric: string, value: number}
  - peer_metrics: {metric: string, percentile: number}
  - user_percentile: number (0-100)
  - comparison: {metric: string, user_vs_peer_percent: number}[]
```

#### 8. Dashboard Metrics
```
GET /v1/analytics/dashboard
Response:
  - summary: {income, expense, net, month-over-month change}
  - top_categories: [{category, percent}, ...]
  - savings_progress: {goal_count, on_track_count, progress_percent}
  - recommendation_impact: {count_implemented, total_savings_paise}
  - compliance: {velocity_usage_percent, flags_count}
  - trends: {category: string, direction: "up"|"down"|"stable"}[]
```

---

### Types

#### TransactionSummary
```typescript
interface TransactionSummary {
  period: "day" | "week" | "month" | "year";
  income_paise: number;
  expense_paise: number;
  net_paise: number;
  category_breakdown: Array<{
    category: string;
    amount_paise: number;
  }>;
  top_spending_categories: Array<{
    category: string;
    percentage: number;
  }>;
  timestamp: string; // RFC3339
}
```

#### SpendingTrend
```typescript
interface SpendingTrend {
  category?: string;
  datapoints: Array<{
    date: string; // RFC3339
    amount_paise: number;
  }>;
  trend_direction: "up" | "down" | "stable";
  avg_daily_paise: number;
  change_percent: number;
}
```

#### BudgetProgress
```typescript
interface BudgetProgress {
  category: string;
  budget_paise: number;
  spent_paise: number;
  remaining_paise: number;
  percentage_used: number;
  status: "on_track" | "warning" | "exceeded";
  projected_end_of_period_paise: number;
}
```

#### SavingsGoal
```typescript
interface SavingsGoal {
  goal_id: string;
  name: string;
  target_paise: number;
  current_paise: number;
  deadline: string; // RFC3339
  progress_percent: number;
  on_track: boolean;
}
```

#### RecommendationImpact
```typescript
interface RecommendationImpact {
  recommendation_id: string;
  estimated_impact_paise: number;
  actual_impact_paise: number;
  accuracy_percent: number;
  implemented_date: string;
  impact_datapoints: Array<{
    date: string;
    cumulative_paise: number;
  }>;
}
```

#### DashboardMetrics
```typescript
interface DashboardMetrics {
  summary: {
    income_paise: number;
    expense_paise: number;
    net_paise: number;
    month_over_month_change_percent: number;
  };
  top_categories: Array<{
    category: string;
    percentage: number;
  }>;
  savings_progress: {
    goal_count: number;
    on_track_count: number;
    progress_percent: number;
  };
  recommendation_impact: {
    count_implemented: number;
    total_savings_paise: number;
  };
  compliance: {
    velocity_usage_percent: number;
    flags_count: number;
  };
}
```

#### Datapoint
```typescript
interface Datapoint {
  date: string; // RFC3339
  amount_paise?: number;
  percentage?: number;
  value?: number;
}
```

#### BenchmarkComparison
```typescript
interface BenchmarkComparison {
  segment: string;
  user_metrics: Record<string, number>;
  peer_metrics: Record<string, number>;
  user_percentile: number;
  comparison: Array<{
    metric: string;
    user_value: number;
    peer_percentile_value: number;
    user_vs_peer_percent: number;
  }>;
}
```

---

### Test Requirements

**Unit Tests** (12 tests):
- [ ] Summary calculation: income - expense = net
- [ ] Trend direction: "up" if datapoints increasing
- [ ] Budget warning at 80% threshold
- [ ] Budget exceeded at 100% threshold
- [ ] Savings goal progress: (current / target) * 100
- [ ] Recommendation impact accuracy: actual / estimated * 100
- [ ] Percentile calculation: rank within peer segment
- [ ] Category breakdown sums to total expense
- [ ] Month-over-month change: (this_month - last_month) / last_month * 100
- [ ] Peer benchmarking respects user segment
- [ ] Dashboard totals match components
- [ ] Trend projection: linear extrapolation from past 30 days

**Integration Tests** (6 tests):
- [ ] Full dashboard loads with all metrics
- [ ] Savings progress updates when goal achieved
- [ ] Recommendation impact reflected in trend
- [ ] Budget warning triggers at threshold
- [ ] Peer comparison respects privacy (no individual profiles exposed)
- [ ] Analytics consistent with Commerce + Compliance data

---

### Validation Rules

**Summary Calculation**:
- `net_paise = income_paise - expense_paise`
- Can be negative (deficit)
- Calculated from actual transactions (no estimates)

**Trend Direction**:
- "up" if most recent 7 datapoints trend higher
- "down" if most recent 7 datapoints trend lower
- "stable" if variance < 5% of average

**Budget Logic**:
- `remaining_paise = budget_paise - spent_paise`
- `status = "exceeded"` if spent > budget
- `status = "warning"` if spent > 80% of budget
- `status = "on_track"` if spent ≤ 80% of budget
- Projected end-of-period = spent + (remaining days / days_elapsed) * spent

**Savings Goals**:
- `progress_percent = (current_paise / target_paise) * 100`
- `on_track = days_remaining > 0 AND (projected completion ≤ deadline)`
- Multiple goals supported per user

**Peer Benchmarking**:
- Segments: age_range (18-25, 26-35, etc.), income_bracket (0-5L, 5-10L, etc.), lifestyle (student, professional, retired)
- User must opt-in (privacy: no anonymized sharing without consent)
- Percentile: (rank / total_in_segment) * 100
- Comparison metrics: income, expense, savings_rate, investment_rate, avg_transaction_size

**Recommendation Impact**:
- Tracked from implementation date onwards
- Compares spending in category before vs after recommendation
- Accuracy = actual / estimated (e.g., estimated ₹10k, actual ₹8k = 80% accuracy)
- Aggregated across all recommendations for dashboard

---

### Success Criteria

✅ All 8 endpoints respond correctly  
✅ All 10 types match TypeScript definitions  
✅ All calculations verified (net, trend, budget, percentile)  
✅ Dashboard assembles from component endpoints  
✅ Peer benchmarking respects privacy (aggregated only)  
✅ Recommendation impact tracking accurate  
✅ All 12 unit tests pass  
✅ All 6 integration tests pass  
✅ Visualizations render correctly (frontend)  

---

### Architecture

**Data Sources** (read-only):
- Commerce (Phase 2): Transaction history, order totals
- Compliance (Phase 3): Velocity limits, AML flags
- Advisor (Phase 7): Recommendation history, impact estimates
- User profile: Budget, goals, preferences

**Computation**:
- Aggregations: SQL GROUP BY (category, date)
- Trends: Linear regression on time series
- Percentiles: PERCENTILE_CONT SQL function
- Projections: Linear extrapolation (y = mx + b)

**Caching**:
- Daily summaries: cached (recalculated at midnight UTC)
- Trends: cached (30-day window, updated hourly)
- Peer benchmarks: cached (monthly recompute)
- Dashboard: composed on-demand from cached components

---

### Frontend Integration

**New Component**: `Analytics` workspace
- Tabs: Summary, Trends, Budget, Goals, Recommendations, Benchmarks
- Charts: Pie (categories), Line (trends), Bar (budget progress), Gauge (goal progress)
- Real-time updates: Subscribe to Commerce transactions, Advisor recommendations

**Styling**: Consistent with existing design tokens (--color-*, --space-*)

---

### Phase 8 Dependencies

**Must Have**:
- ✅ Phase 2 (Commerce): Transaction data
- ✅ Phase 3 (Compliance): Velocity context
- ✅ Phase 7 (Advisor): Recommendation impact data

**Nice to Have**:
- Market data API (external integration)
- Peer benchmarking database (anonymized)
- Export to CSV/PDF

---

### Compliance & Standards

**Privacy**:
- No individual user profiles exposed in peer benchmarks
- Aggregated statistics only
- User must opt-in to benchmarking comparison

**RBI Alignment**:
- Sutra 7 (Compliance): Velocity usage transparency
- Recommendation 6 (Reporting): Regular insights to user

---

### Timeline

**Week 1**:
- [ ] Spec review + approval
- [ ] Implement backend endpoints (4 endpoints)
- [ ] Unit tests passing
- [ ] TypeScript types created

**Week 2**:
- [ ] Implement remaining 4 endpoints
- [ ] Integrate with Commerce + Advisor
- [ ] Dashboard component in frontend
- [ ] All tests passing

**Week 3**:
- [ ] Performance optimization (caching)
- [ ] Peer benchmarking (privacy review)
- [ ] Production deployment
- [ ] Monitoring + alerts

---

### Success Metrics (Week 1)

- [ ] Spec approved
- [ ] 4 endpoints working (summary, trends, budget, goals)
- [ ] All unit tests passing
- [ ] Dashboard displays data
- [ ] No regressions in other phases

---

## Next Phase (Phase 9 Suggested)

**Automation Workspace**: Rules engine for automatic actions
- Rule builder: IF condition THEN action
- Examples:
  - "If category spending > budget, send alert"
  - "If recommendation detected, auto-implement"
  - "If velocity > 80%, pause transactions"
  - "If new goal, auto-allocate savings"

This would create autonomous financial management.

---

## Roadmap

```
Phase 0+1: Foundation
Phase 2: Commerce (orders, payments)
Phase 3: Compliance (KYC, AML)
Phase 4: Governance (safety, audit)
Phase 5: Evaluation (judges, coverage)
Phase 6: Assistant (conversational AI)
Phase 7: Advisor (recommendations) 🔜 In spec-first development
Phase 8: Analytics (dashboards) 📋 This spec
Phase 9: Automation (rules engine) 🚀 Next
```

Platform provides end-to-end financial management: **Advisory → Analytics → Automation**
