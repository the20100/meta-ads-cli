package api

import (
	"encoding/json"
	"fmt"
)

// FlexString handles Meta API fields that are inconsistently returned as
// either a JSON string ("1500") or a JSON number (1500).
// This fixes parsing errors like: cannot unmarshal number into Go struct field ... of type string
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	// Try number (int or float)
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		if n == float64(int64(n)) {
			*f = FlexString(fmt.Sprintf("%d", int64(n)))
		} else {
			*f = FlexString(fmt.Sprintf("%g", n))
		}
		return nil
	}
	// null
	*f = ""
	return nil
}

func (f FlexString) String() string {
	return string(f)
}

// MetaError wraps a Meta API error response.
type MetaError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Subcode int    `json:"error_subcode"`
}

func (e *MetaError) Error() string {
	if e.Subcode != 0 {
		return "meta api error " + itoa(e.Code) + " (subcode " + itoa(e.Subcode) + "): " + e.Message
	}
	return "meta api error " + itoa(e.Code) + ": " + e.Message
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 12)
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	tmp := make([]byte, 0, 12)
	for n > 0 {
		tmp = append(tmp, byte('0'+n%10))
		n /= 10
	}
	for i := len(tmp) - 1; i >= 0; i-- {
		b = append(b, tmp[i])
	}
	return string(b)
}

// Paging wraps the paging field in list responses.
type Paging struct {
	Cursors *struct {
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"cursors,omitempty"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// Account represents a Meta Ad Account.
type Account struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Currency    string `json:"currency"`
	Status      int    `json:"account_status"`
	TimezoneName string `json:"timezone_name"`
	AmountSpent string `json:"amount_spent,omitempty"`
	Balance     string `json:"balance,omitempty"`
}

// Campaign represents a Meta campaign.
type Campaign struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	EffectiveStatus string `json:"effective_status,omitempty"`
	Objective       string `json:"objective"`
	DailyBudget     string `json:"daily_budget,omitempty"`
	LifetimeBudget  string `json:"lifetime_budget,omitempty"`
	BudgetRemaining string `json:"budget_remaining,omitempty"`
	BidStrategy     string `json:"bid_strategy,omitempty"`
	StartTime       string `json:"start_time,omitempty"`
	StopTime        string `json:"stop_time,omitempty"`
	CreatedTime     string `json:"created_time,omitempty"`
	UpdatedTime     string `json:"updated_time,omitempty"`
}

// AdSet represents a Meta ad set.
type AdSet struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	EffectiveStatus string     `json:"effective_status,omitempty"`
	CampaignID      string     `json:"campaign_id"`
	DailyBudget     FlexString `json:"daily_budget,omitempty"`
	LifetimeBudget  FlexString `json:"lifetime_budget,omitempty"`
	BudgetRemaining FlexString `json:"budget_remaining,omitempty"`
	BidAmount       FlexString `json:"bid_amount,omitempty"`
	BidStrategy     string     `json:"bid_strategy,omitempty"`
	BillingEvent    string     `json:"billing_event,omitempty"`
	OptimizationGoal string    `json:"optimization_goal,omitempty"`
	StartTime       string     `json:"start_time,omitempty"`
	EndTime         string     `json:"end_time,omitempty"`
	CreatedTime     string     `json:"created_time,omitempty"`
	UpdatedTime     string     `json:"updated_time,omitempty"`
	DestinationType string     `json:"destination_type,omitempty"`
	// Nested campaign info (returned when requesting campaign{id,name,objective})
	Campaign *struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Objective string `json:"objective"`
	} `json:"campaign,omitempty"`
	// Complex fields returned as raw JSON for flexible display
	Targeting      json.RawMessage `json:"targeting,omitempty"`
	PromotedObject json.RawMessage `json:"promoted_object,omitempty"`
	AttributionSpec json.RawMessage `json:"attribution_spec,omitempty"`
	PacingType     json.RawMessage `json:"pacing_type,omitempty"`
}

// Ad represents a Meta ad.
type Ad struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Status          string          `json:"status"`
	EffectiveStatus string          `json:"effective_status,omitempty"`
	AdSetID         string          `json:"adset_id"`
	CampaignID      string          `json:"campaign_id"`
	Creative        json.RawMessage `json:"creative,omitempty"`
	CreatedTime     string          `json:"created_time,omitempty"`
	UpdatedTime     string          `json:"updated_time,omitempty"`
}

// Insight represents a row of Meta performance data.
// Fields are dynamic based on requested metrics, so we use raw JSON.
type Insight = json.RawMessage

// Audience represents a Meta custom audience.
type Audience struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	Subtype                   string `json:"subtype"`
	ApproximateCountLowerBound int    `json:"approximate_count_lower_bound,omitempty"`
	ApproximateCountUpperBound int    `json:"approximate_count_upper_bound,omitempty"`
	DeliveryStatus            *struct {
		Code        int    `json:"code"`
		Description string `json:"description"`
	} `json:"delivery_status,omitempty"`
	Description        string          `json:"description,omitempty"`
	TimeContentUpdated string          `json:"time_content_updated,omitempty"`
	Rule               json.RawMessage `json:"rule,omitempty"`
	RuleAggregation    json.RawMessage `json:"rule_aggregation,omitempty"`
	RetentionDays      int             `json:"retention_days,omitempty"`
	PixelID            string          `json:"pixel_id,omitempty"`
	TimeCreated        string          `json:"time_created,omitempty"`
	TimeUpdated        string          `json:"time_updated,omitempty"`
}

// Pixel represents a Meta pixel.
type Pixel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LastFiredTime string `json:"last_fired_time,omitempty"`
	CreationTime  string `json:"creation_time,omitempty"`
	IsUnavailable bool   `json:"is_unavailable,omitempty"`
}

// User is returned by GET /me.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}
