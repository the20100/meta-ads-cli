package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── Flags ────────────────────────────────────────────────────────────────────

var (
	auditPeriod string
	auditStart  string
	auditEnd    string
	auditAll    bool
	auditFormat string
	auditOutput string
)

// ── Command ──────────────────────────────────────────────────────────────────

var auditExportCmd = &cobra.Command{
	Use:   "audit-export",
	Short: "Export a full account audit (campaigns, ad sets, ads with config & metrics)",
	Long: `Export a complete audit of a Meta Ads account as a structured document.

Includes campaign/adset/ad configuration (budgets, targeting, creative) and
performance metrics (spend, impressions, ROAS, etc.) for the specified period.

Examples:
  # Default: last 3 months, active only, JSON to stdout
  meta-ads audit-export -a act_123456789

  # Last 30 days, save to file
  meta-ads audit-export -a act_123456789 --period 30d -o audit.json

  # Custom date range, CSV format
  meta-ads audit-export -a act_123456789 --start 2026-01-01 --end 2026-02-01 --format csv -o audit.csv

  # Include everything (even zero-impression items)
  meta-ads audit-export -a act_123456789 --all

  # Markdown report
  meta-ads audit-export -a act_123456789 --format md -o audit.md`,
	RunE: runAuditExport,
}

func init() {
	auditExportCmd.Flags().StringVar(&auditPeriod, "period", "3m", "Time period: 7d, 30d, 3m, 6m, 1y")
	auditExportCmd.Flags().StringVar(&auditStart, "start", "", "Start date YYYY-MM-DD (overrides --period)")
	auditExportCmd.Flags().StringVar(&auditEnd, "end", "", "End date YYYY-MM-DD (overrides --period)")
	auditExportCmd.Flags().BoolVar(&auditAll, "all", false, "Include all items (even with zero impressions)")
	auditExportCmd.Flags().StringVar(&auditFormat, "format", "json", "Output format: json, csv, md")
	auditExportCmd.Flags().StringVarP(&auditOutput, "output", "o", "", "Output file path (stdout if omitted)")

	rootCmd.AddCommand(auditExportCmd)
}

// ── Types ────────────────────────────────────────────────────────────────────

type auditReport struct {
	AccountID  string            `json:"account_id"`
	Period     auditPeriodRange  `json:"period"`
	ExportedAt string            `json:"exported_at"`
	Campaigns  []auditCampaign   `json:"campaigns"`
}

type auditPeriodRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type auditCampaign struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Status          string        `json:"status"`
	EffectiveStatus string        `json:"effective_status"`
	Objective       string        `json:"objective"`
	DailyBudget     string        `json:"daily_budget,omitempty"`
	LifetimeBudget  string        `json:"lifetime_budget,omitempty"`
	BudgetRemaining string        `json:"budget_remaining,omitempty"`
	BidStrategy     string        `json:"bid_strategy,omitempty"`
	StartTime       string        `json:"start_time,omitempty"`
	StopTime        string        `json:"stop_time,omitempty"`
	Metrics         *auditMetrics `json:"metrics,omitempty"`
	AdSets          []auditAdSet  `json:"adsets,omitempty"`
}

type auditAdSet struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Status          string          `json:"status"`
	EffectiveStatus string          `json:"effective_status"`
	CampaignID      string          `json:"campaign_id"`
	Config          auditAdSetCfg   `json:"config"`
	Targeting       json.RawMessage `json:"targeting,omitempty"`
	Metrics         *auditMetrics   `json:"metrics,omitempty"`
	Ads             []auditAd       `json:"ads,omitempty"`
}

type auditAdSetCfg struct {
	DailyBudget      string          `json:"daily_budget,omitempty"`
	LifetimeBudget   string          `json:"lifetime_budget,omitempty"`
	BudgetRemaining  string          `json:"budget_remaining,omitempty"`
	BidAmount        string          `json:"bid_amount,omitempty"`
	BidStrategy      string          `json:"bid_strategy,omitempty"`
	BillingEvent     string          `json:"billing_event,omitempty"`
	OptimizationGoal string          `json:"optimization_goal,omitempty"`
	DestinationType  string          `json:"destination_type,omitempty"`
	StartTime        string          `json:"start_time,omitempty"`
	EndTime          string          `json:"end_time,omitempty"`
	PromotedObject   json.RawMessage `json:"promoted_object,omitempty"`
	AttributionSpec  json.RawMessage `json:"attribution_spec,omitempty"`
	PacingType       json.RawMessage `json:"pacing_type,omitempty"`
}

type auditAd struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Status          string          `json:"status"`
	EffectiveStatus string          `json:"effective_status"`
	AdSetID         string          `json:"adset_id"`
	CampaignID      string          `json:"campaign_id"`
	Creative        json.RawMessage `json:"creative,omitempty"`
	Metrics         *auditMetrics   `json:"metrics,omitempty"`
}

type auditMetrics struct {
	Spend             string `json:"spend"`
	Impressions       string `json:"impressions"`
	Reach             string `json:"reach"`
	CPM               string `json:"cpm"`
	Frequency         string `json:"frequency"`
	LinkClicks        string `json:"link_clicks"`
	CTR               string `json:"ctr"`
	VideoViews3s      string `json:"video_views_3s"`
	VideoViews15s     string `json:"video_views_15s"`
	ThruPlay          string `json:"thruplay"`
	HookRatio         string `json:"hook_ratio"`
	HoldRate          string `json:"hold_rate"`
	AddToCart         string `json:"add_to_cart"`
	CostPerAddToCart  string `json:"cost_per_add_to_cart"`
	Purchases         string `json:"purchases"`
	CostPerPurchase   string `json:"cost_per_purchase"`
	PurchaseValue     string `json:"purchase_value"`
	ROAS              string `json:"roas"`
	ConversionRate    string `json:"conversion_rate"`
	EngagementRate    string `json:"engagement_rate"`
	Leads             string `json:"leads"`
	CostPerLead       string `json:"cost_per_lead"`
}

// actionEntry is used to parse the actions / action_values / cost_per_action_type arrays.
type actionEntry struct {
	ActionType string `json:"action_type"`
	Value      string `json:"value"`
}

// ── Insight fields requested from Meta API ───────────────────────────────────

// video_3_sec_watched_actions does NOT exist in the API — 3s views come from
// the "video_view" action_type inside the "actions" array.
// video_15_sec_watched_actions is the closest named field; ThruPlay (≥15s or
// complete) is video_thruplay_watched_actions.
const auditInsightFields = "spend,impressions,reach,cpm,frequency,inline_link_clicks,inline_link_click_ctr,video_15_sec_watched_actions,video_thruplay_watched_actions,actions,action_values,cost_per_action_type,purchase_roas"

// ── Main runner ──────────────────────────────────────────────────────────────

func runAuditExport(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	startDate, endDate, err := resolveAuditDateRange()
	if err != nil {
		return err
	}
	timeRange := fmt.Sprintf(`{"since":"%s","until":"%s"}`, startDate, endDate)

	// ── 1. Fetch campaigns ───────────────────────────────────────────────
	progress("Fetching campaigns...")
	campaignFields := "id,name,status,effective_status,objective,daily_budget,lifetime_budget,budget_remaining,bid_strategy,start_time,stop_time,created_time,updated_time"
	campParams := url.Values{}
	campParams.Set("fields", campaignFields)
	campItems, err := client.GetAll("/"+account+"/campaigns", campParams)
	if err != nil {
		return fmt.Errorf("fetching campaigns: %w", err)
	}
	progress("  found %d campaigns", len(campItems))

	// ── 2. Fetch campaign insights ───────────────────────────────────────
	progress("Fetching campaign insights...")
	campInsights, err := fetchInsightsMap(account, "campaign", timeRange)
	if err != nil {
		return fmt.Errorf("fetching campaign insights: %w", err)
	}

	// ── 3. Fetch ad sets ─────────────────────────────────────────────────
	progress("Fetching ad sets...")
	adsetFields := "id,name,status,effective_status,campaign_id,daily_budget,lifetime_budget,budget_remaining,bid_amount,bid_strategy,billing_event,optimization_goal,start_time,end_time,created_time,updated_time,destination_type,targeting,promoted_object,attribution_spec,pacing_type"
	asParams := url.Values{}
	asParams.Set("fields", adsetFields)
	asItems, err := client.GetAll("/"+account+"/adsets", asParams)
	if err != nil {
		return fmt.Errorf("fetching adsets: %w", err)
	}
	progress("  found %d ad sets", len(asItems))

	// ── 4. Fetch ad set insights ─────────────────────────────────────────
	progress("Fetching ad set insights...")
	adsetInsights, err := fetchInsightsMap(account, "adset", timeRange)
	if err != nil {
		return fmt.Errorf("fetching adset insights: %w", err)
	}

	// ── 5. Fetch ads ─────────────────────────────────────────────────────
	progress("Fetching ads...")
	adFields := "id,name,status,effective_status,adset_id,campaign_id,creative{id,body,title,call_to_action_type,link_url,image_url,thumbnail_url,video_id,object_story_spec,asset_feed_spec,effective_object_story_id},created_time,updated_time"
	adParams := url.Values{}
	adParams.Set("fields", adFields)
	adRawItems, err := client.GetAll("/"+account+"/ads", adParams)
	if err != nil {
		return fmt.Errorf("fetching ads: %w", err)
	}
	progress("  found %d ads", len(adRawItems))

	// ── 6. Fetch ad insights ─────────────────────────────────────────────
	progress("Fetching ad insights...")
	adInsights, err := fetchInsightsMap(account, "ad", timeRange)
	if err != nil {
		return fmt.Errorf("fetching ad insights: %w", err)
	}

	// ── 7. Build report structure ────────────────────────────────────────
	progress("Building report...")

	// Parse ads
	type rawAd struct {
		ID              string          `json:"id"`
		Name            string          `json:"name"`
		Status          string          `json:"status"`
		EffectiveStatus string          `json:"effective_status"`
		AdSetID         string          `json:"adset_id"`
		CampaignID      string          `json:"campaign_id"`
		Creative        json.RawMessage `json:"creative"`
		CreatedTime     string          `json:"created_time"`
		UpdatedTime     string          `json:"updated_time"`
	}
	adsByAdSet := map[string][]auditAd{}
	for _, raw := range adRawItems {
		var a rawAd
		if err := json.Unmarshal(raw, &a); err != nil {
			continue
		}
		aa := auditAd{
			ID:              a.ID,
			Name:            a.Name,
			Status:          a.Status,
			EffectiveStatus: a.EffectiveStatus,
			AdSetID:         a.AdSetID,
			CampaignID:      a.CampaignID,
			Creative:        a.Creative,
			Metrics:         adInsights[a.ID],
		}
		if !auditAll && aa.Metrics == nil {
			continue
		}
		if !auditAll && aa.Metrics != nil && aa.Metrics.Impressions == "0" {
			continue
		}
		adsByAdSet[a.AdSetID] = append(adsByAdSet[a.AdSetID], aa)
	}

	// Parse adsets
	type rawAdSet struct {
		ID               string          `json:"id"`
		Name             string          `json:"name"`
		Status           string          `json:"status"`
		EffectiveStatus  string          `json:"effective_status"`
		CampaignID       string          `json:"campaign_id"`
		DailyBudget      json.RawMessage `json:"daily_budget"`
		LifetimeBudget   json.RawMessage `json:"lifetime_budget"`
		BudgetRemaining  json.RawMessage `json:"budget_remaining"`
		BidAmount        json.RawMessage `json:"bid_amount"`
		BidStrategy      string          `json:"bid_strategy"`
		BillingEvent     string          `json:"billing_event"`
		OptimizationGoal string          `json:"optimization_goal"`
		StartTime        string          `json:"start_time"`
		EndTime          string          `json:"end_time"`
		DestinationType  string          `json:"destination_type"`
		Targeting        json.RawMessage `json:"targeting"`
		PromotedObject   json.RawMessage `json:"promoted_object"`
		AttributionSpec  json.RawMessage `json:"attribution_spec"`
		PacingType       json.RawMessage `json:"pacing_type"`
	}
	adsetsByCampaign := map[string][]auditAdSet{}
	for _, raw := range asItems {
		var a rawAdSet
		if err := json.Unmarshal(raw, &a); err != nil {
			continue
		}
		as := auditAdSet{
			ID:              a.ID,
			Name:            a.Name,
			Status:          a.Status,
			EffectiveStatus: a.EffectiveStatus,
			CampaignID:      a.CampaignID,
			Config: auditAdSetCfg{
				DailyBudget:      flexStr(a.DailyBudget),
				LifetimeBudget:   flexStr(a.LifetimeBudget),
				BudgetRemaining:  flexStr(a.BudgetRemaining),
				BidAmount:        flexStr(a.BidAmount),
				BidStrategy:      a.BidStrategy,
				BillingEvent:     a.BillingEvent,
				OptimizationGoal: a.OptimizationGoal,
				DestinationType:  a.DestinationType,
				StartTime:        a.StartTime,
				EndTime:          a.EndTime,
				PromotedObject:   a.PromotedObject,
				AttributionSpec:  a.AttributionSpec,
				PacingType:       a.PacingType,
			},
			Targeting: a.Targeting,
			Metrics:   adsetInsights[a.ID],
			Ads:       adsByAdSet[a.ID],
		}
		if !auditAll && as.Metrics == nil && len(as.Ads) == 0 {
			continue
		}
		adsetsByCampaign[a.CampaignID] = append(adsetsByCampaign[a.CampaignID], as)
	}

	// Parse campaigns
	type rawCampaign struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		Status          string `json:"status"`
		EffectiveStatus string `json:"effective_status"`
		Objective       string `json:"objective"`
		DailyBudget     string `json:"daily_budget"`
		LifetimeBudget  string `json:"lifetime_budget"`
		BudgetRemaining string `json:"budget_remaining"`
		BidStrategy     string `json:"bid_strategy"`
		StartTime       string `json:"start_time"`
		StopTime        string `json:"stop_time"`
	}
	var campaigns []auditCampaign
	for _, raw := range campItems {
		var c rawCampaign
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}
		ac := auditCampaign{
			ID:              c.ID,
			Name:            c.Name,
			Status:          c.Status,
			EffectiveStatus: c.EffectiveStatus,
			Objective:       c.Objective,
			DailyBudget:     c.DailyBudget,
			LifetimeBudget:  c.LifetimeBudget,
			BudgetRemaining: c.BudgetRemaining,
			BidStrategy:     c.BidStrategy,
			StartTime:       c.StartTime,
			StopTime:        c.StopTime,
			Metrics:         campInsights[c.ID],
			AdSets:          adsetsByCampaign[c.ID],
		}
		if !auditAll && ac.Metrics == nil && len(ac.AdSets) == 0 {
			continue
		}
		campaigns = append(campaigns, ac)
	}

	report := auditReport{
		AccountID:  account,
		Period:     auditPeriodRange{Start: startDate, End: endDate},
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Campaigns:  campaigns,
	}

	// ── 8. Output ────────────────────────────────────────────────────────
	totalAds := 0
	totalAdSets := 0
	for _, c := range campaigns {
		totalAdSets += len(c.AdSets)
		for _, as := range c.AdSets {
			totalAds += len(as.Ads)
		}
	}
	progress("Exporting %d campaigns, %d ad sets, %d ads", len(campaigns), totalAdSets, totalAds)

	return writeAuditOutput(report)
}

// ── Date range resolution ────────────────────────────────────────────────────

func resolveAuditDateRange() (string, string, error) {
	if auditStart != "" && auditEnd != "" {
		return auditStart, auditEnd, nil
	}
	if auditStart != "" || auditEnd != "" {
		return "", "", fmt.Errorf("both --start and --end must be provided together")
	}
	return parsePeriod(auditPeriod)
}

func parsePeriod(p string) (string, string, error) {
	now := time.Now()
	end := now.Format("2006-01-02")
	p = strings.ToLower(strings.TrimSpace(p))

	switch p {
	case "7d":
		return now.AddDate(0, 0, -7).Format("2006-01-02"), end, nil
	case "14d":
		return now.AddDate(0, 0, -14).Format("2006-01-02"), end, nil
	case "30d":
		return now.AddDate(0, 0, -30).Format("2006-01-02"), end, nil
	case "60d":
		return now.AddDate(0, 0, -60).Format("2006-01-02"), end, nil
	case "90d", "3m", "3month", "3months":
		return now.AddDate(0, -3, 0).Format("2006-01-02"), end, nil
	case "6m", "6month", "6months":
		return now.AddDate(0, -6, 0).Format("2006-01-02"), end, nil
	case "1y", "12m", "12month", "12months":
		return now.AddDate(-1, 0, 0).Format("2006-01-02"), end, nil
	default:
		return "", "", fmt.Errorf("invalid period %q — use 7d, 30d, 3m, 6m, 1y, etc.", p)
	}
}

// ── Insight fetching ─────────────────────────────────────────────────────────

// fetchInsightsMap fetches insights for the given level and returns a map of object_id → metrics.
func fetchInsightsMap(account, level, timeRange string) (map[string]*auditMetrics, error) {
	// Build the fields: we need the object ID field + all metric fields
	idField := level + "_id"
	fields := idField + "," + auditInsightFields

	params := url.Values{}
	params.Set("fields", fields)
	params.Set("level", level)
	params.Set("time_range", timeRange)
	params.Set("limit", "500")

	items, err := client.GetAll("/"+account+"/insights", params)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*auditMetrics, len(items))
	for _, raw := range items {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}

		objectID := jsonString(m[idField])
		if objectID == "" {
			continue
		}

		metrics := buildMetrics(m)
		result[objectID] = metrics
	}

	return result, nil
}

// ── Metric building ──────────────────────────────────────────────────────────

func buildMetrics(m map[string]json.RawMessage) *auditMetrics {
	// Direct fields
	spend := jsonString(m["spend"])
	impressions := jsonString(m["impressions"])
	reach := jsonString(m["reach"])
	cpm := jsonString(m["cpm"])
	frequency := jsonString(m["frequency"])
	linkClicks := jsonString(m["inline_link_clicks"])
	ctr := jsonString(m["inline_link_click_ctr"])

	// Video views
	// 3-second video views = "video_view" action_type inside the "actions" array
	// 15-second views = video_15_sec_watched_actions (dedicated field)
	// ThruPlay (≥15s or complete) = video_thruplay_watched_actions (dedicated field)
	video15s := sumActionEntries(m["video_15_sec_watched_actions"])
	thruplay := sumActionEntries(m["video_thruplay_watched_actions"])

	// Actions
	actions := parseActionEntries(m["actions"])
	video3s := findAction(actions, "video_view")
	addToCart := findAction(actions, "add_to_cart", "offsite_conversion.fb_pixel_add_to_cart")
	purchases := findAction(actions, "purchase", "offsite_conversion.fb_pixel_purchase")
	leads := findAction(actions, "lead", "offsite_conversion.fb_pixel_lead")
	postEngagement := findAction(actions, "post_engagement")

	// Action values
	actionValues := parseActionEntries(m["action_values"])
	purchaseValue := findAction(actionValues, "purchase", "offsite_conversion.fb_pixel_purchase")

	// Cost per action
	costPerActions := parseActionEntries(m["cost_per_action_type"])
	costPerATC := findAction(costPerActions, "add_to_cart", "offsite_conversion.fb_pixel_add_to_cart")
	costPerPurchase := findAction(costPerActions, "purchase", "offsite_conversion.fb_pixel_purchase")
	costPerLead := findAction(costPerActions, "lead", "offsite_conversion.fb_pixel_lead")

	// ROAS
	roas := ""
	if raw, ok := m["purchase_roas"]; ok {
		var entries []actionEntry
		if json.Unmarshal(raw, &entries) == nil && len(entries) > 0 {
			roas = entries[0].Value
		}
	}
	if roas == "" {
		roas = computeRatio(purchaseValue, spend)
	}

	return &auditMetrics{
		Spend:            spend,
		Impressions:      impressions,
		Reach:            reach,
		CPM:              cpm,
		Frequency:        frequency,
		LinkClicks:       linkClicks,
		CTR:              ctr,
		VideoViews3s:     video3s,
		VideoViews15s:    video15s,
		ThruPlay:         thruplay,
		HookRatio:        computeRatio(video3s, impressions),
		HoldRate:         computeRatio(thruplay, video3s),
		AddToCart:        addToCart,
		CostPerAddToCart: costPerATC,
		Purchases:        purchases,
		CostPerPurchase:  costPerPurchase,
		PurchaseValue:    purchaseValue,
		ROAS:             roas,
		ConversionRate:   computeRatio(purchases, linkClicks),
		EngagementRate:   computeRatio(postEngagement, impressions),
		Leads:            leads,
		CostPerLead:      costPerLead,
	}
}

// ── Action helpers ───────────────────────────────────────────────────────────

func parseActionEntries(raw json.RawMessage) []actionEntry {
	if len(raw) == 0 {
		return nil
	}
	var entries []actionEntry
	json.Unmarshal(raw, &entries)
	return entries
}

// sumActionEntries sums all entries in a video action field (they sometimes have one entry per action_type).
func sumActionEntries(raw json.RawMessage) string {
	entries := parseActionEntries(raw)
	if len(entries) == 0 {
		return "0"
	}
	total := 0
	for _, e := range entries {
		n, _ := strconv.Atoi(e.Value)
		total += n
	}
	return strconv.Itoa(total)
}

// findAction looks for the first matching action_type and returns its value.
func findAction(entries []actionEntry, types ...string) string {
	for _, t := range types {
		for _, e := range entries {
			if e.ActionType == t {
				return e.Value
			}
		}
	}
	return "0"
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "0"
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try as number
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		if n == float64(int64(n)) {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return "0"
}

func flexStr(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		if n == float64(int64(n)) {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return ""
}

func computeRatio(numerator, denominator string) string {
	num, err1 := strconv.ParseFloat(numerator, 64)
	den, err2 := strconv.ParseFloat(denominator, 64)
	if err1 != nil || err2 != nil || den == 0 {
		return "0"
	}
	return fmt.Sprintf("%.4f", num/den)
}

// ── Output ───────────────────────────────────────────────────────────────────

func writeAuditOutput(report auditReport) error {
	var w *os.File
	if auditOutput != "" {
		f, err := os.Create(auditOutput)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		w = f
	} else {
		w = os.Stdout
	}

	switch strings.ToLower(auditFormat) {
	case "json":
		return writeAuditJSON(w, report)
	case "csv":
		return writeAuditCSV(w, report)
	case "md", "markdown":
		return writeAuditMarkdown(w, report)
	default:
		return fmt.Errorf("unsupported format %q — use json, csv, or md", auditFormat)
	}
}

func writeAuditJSON(w *os.File, report auditReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeAuditCSV(w *os.File, report auditReport) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	headers := []string{
		"Campaign ID", "Campaign Name", "Campaign Status", "Campaign Objective",
		"Campaign Daily Budget", "Campaign Lifetime Budget", "Campaign Bid Strategy",
		"AdSet ID", "AdSet Name", "AdSet Status",
		"AdSet Daily Budget", "AdSet Lifetime Budget", "AdSet Bid Strategy",
		"AdSet Billing Event", "AdSet Optimization Goal", "AdSet Destination Type",
		"Ad ID", "Ad Name", "Ad Status",
		"Spend", "Impressions", "Reach", "CPM", "Frequency",
		"Link Clicks", "CTR",
		"Video Views 3s", "Video Views 15s", "ThruPlay", "Hook Ratio", "Hold Rate",
		"Add to Cart", "Cost per Add to Cart",
		"Purchases", "Cost per Purchase", "Purchase Value", "ROAS", "Conversion Rate",
		"Engagement Rate",
		"Leads", "Cost per Lead",
	}
	if err := cw.Write(headers); err != nil {
		return err
	}

	for _, c := range report.Campaigns {
		for _, as := range c.AdSets {
			if len(as.Ads) == 0 {
				// Write adset row with no ad
				row := buildCSVRow(c, as, auditAd{}, as.Metrics)
				if err := cw.Write(row); err != nil {
					return err
				}
				continue
			}
			for _, ad := range as.Ads {
				row := buildCSVRow(c, as, ad, ad.Metrics)
				if err := cw.Write(row); err != nil {
					return err
				}
			}
		}
		// Campaign with no adsets
		if len(c.AdSets) == 0 {
			row := buildCSVRow(c, auditAdSet{}, auditAd{}, c.Metrics)
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildCSVRow(c auditCampaign, as auditAdSet, ad auditAd, metrics *auditMetrics) []string {
	m := metrics
	if m == nil {
		m = &auditMetrics{}
	}
	return []string{
		c.ID, c.Name, c.EffectiveStatus, c.Objective,
		c.DailyBudget, c.LifetimeBudget, c.BidStrategy,
		as.ID, as.Name, as.EffectiveStatus,
		as.Config.DailyBudget, as.Config.LifetimeBudget, as.Config.BidStrategy,
		as.Config.BillingEvent, as.Config.OptimizationGoal, as.Config.DestinationType,
		ad.ID, ad.Name, ad.EffectiveStatus,
		m.Spend, m.Impressions, m.Reach, m.CPM, m.Frequency,
		m.LinkClicks, m.CTR,
		m.VideoViews3s, m.VideoViews15s, m.ThruPlay, m.HookRatio, m.HoldRate,
		m.AddToCart, m.CostPerAddToCart,
		m.Purchases, m.CostPerPurchase, m.PurchaseValue, m.ROAS, m.ConversionRate,
		m.EngagementRate,
		m.Leads, m.CostPerLead,
	}
}

func writeAuditMarkdown(w *os.File, report auditReport) error {
	fmt.Fprintf(w, "# Meta Ads Audit Report\n\n")
	fmt.Fprintf(w, "- **Account:** %s\n", report.AccountID)
	fmt.Fprintf(w, "- **Period:** %s → %s\n", report.Period.Start, report.Period.End)
	fmt.Fprintf(w, "- **Exported:** %s\n\n", report.ExportedAt)

	for _, c := range report.Campaigns {
		fmt.Fprintf(w, "---\n\n")
		fmt.Fprintf(w, "## Campaign: %s\n\n", c.Name)
		fmt.Fprintf(w, "| Field | Value |\n|---|---|\n")
		fmt.Fprintf(w, "| ID | %s |\n", c.ID)
		fmt.Fprintf(w, "| Status | %s |\n", c.EffectiveStatus)
		fmt.Fprintf(w, "| Objective | %s |\n", c.Objective)
		fmt.Fprintf(w, "| Daily Budget | %s |\n", c.DailyBudget)
		fmt.Fprintf(w, "| Lifetime Budget | %s |\n", c.LifetimeBudget)
		fmt.Fprintf(w, "| Bid Strategy | %s |\n", c.BidStrategy)
		if c.Metrics != nil {
			fmt.Fprintf(w, "\n### Campaign Metrics\n\n")
			writeMetricsTable(w, c.Metrics)
		}

		for _, as := range c.AdSets {
			fmt.Fprintf(w, "\n### Ad Set: %s\n\n", as.Name)
			fmt.Fprintf(w, "| Field | Value |\n|---|---|\n")
			fmt.Fprintf(w, "| ID | %s |\n", as.ID)
			fmt.Fprintf(w, "| Status | %s |\n", as.EffectiveStatus)
			fmt.Fprintf(w, "| Daily Budget | %s |\n", as.Config.DailyBudget)
			fmt.Fprintf(w, "| Lifetime Budget | %s |\n", as.Config.LifetimeBudget)
			fmt.Fprintf(w, "| Bid Strategy | %s |\n", as.Config.BidStrategy)
			fmt.Fprintf(w, "| Billing Event | %s |\n", as.Config.BillingEvent)
			fmt.Fprintf(w, "| Optimization Goal | %s |\n", as.Config.OptimizationGoal)
			fmt.Fprintf(w, "| Destination Type | %s |\n", as.Config.DestinationType)

			if len(as.Targeting) > 0 {
				fmt.Fprintf(w, "\n**Targeting:**\n```json\n")
				var v any
				if json.Unmarshal(as.Targeting, &v) == nil {
					b, _ := json.MarshalIndent(v, "", "  ")
					fmt.Fprintf(w, "%s\n", string(b))
				}
				fmt.Fprintf(w, "```\n")
			}

			if as.Metrics != nil {
				fmt.Fprintf(w, "\n#### Ad Set Metrics\n\n")
				writeMetricsTable(w, as.Metrics)
			}

			for _, ad := range as.Ads {
				fmt.Fprintf(w, "\n#### Ad: %s\n\n", ad.Name)
				fmt.Fprintf(w, "| Field | Value |\n|---|---|\n")
				fmt.Fprintf(w, "| ID | %s |\n", ad.ID)
				fmt.Fprintf(w, "| Status | %s |\n", ad.EffectiveStatus)

				if len(ad.Creative) > 0 {
					fmt.Fprintf(w, "\n**Creative:**\n```json\n")
					var v any
					if json.Unmarshal(ad.Creative, &v) == nil {
						b, _ := json.MarshalIndent(v, "", "  ")
						fmt.Fprintf(w, "%s\n", string(b))
					}
					fmt.Fprintf(w, "```\n")
				}

				if ad.Metrics != nil {
					fmt.Fprintf(w, "\n##### Ad Metrics\n\n")
					writeMetricsTable(w, ad.Metrics)
				}
			}
		}
		fmt.Fprintf(w, "\n")
	}

	return nil
}

func writeMetricsTable(w *os.File, m *auditMetrics) {
	fmt.Fprintf(w, "| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(w, "| Spend | %s |\n", m.Spend)
	fmt.Fprintf(w, "| Impressions | %s |\n", m.Impressions)
	fmt.Fprintf(w, "| Reach | %s |\n", m.Reach)
	fmt.Fprintf(w, "| CPM | %s |\n", m.CPM)
	fmt.Fprintf(w, "| Frequency | %s |\n", m.Frequency)
	fmt.Fprintf(w, "| Link Clicks | %s |\n", m.LinkClicks)
	fmt.Fprintf(w, "| CTR | %s |\n", m.CTR)
	fmt.Fprintf(w, "| Video Views 3s | %s |\n", m.VideoViews3s)
	fmt.Fprintf(w, "| Video Views 15s | %s |\n", m.VideoViews15s)
	fmt.Fprintf(w, "| ThruPlay | %s |\n", m.ThruPlay)
	fmt.Fprintf(w, "| Hook Ratio | %s |\n", m.HookRatio)
	fmt.Fprintf(w, "| Hold Rate | %s |\n", m.HoldRate)
	fmt.Fprintf(w, "| Add to Cart | %s |\n", m.AddToCart)
	fmt.Fprintf(w, "| Cost/Add to Cart | %s |\n", m.CostPerAddToCart)
	fmt.Fprintf(w, "| Purchases | %s |\n", m.Purchases)
	fmt.Fprintf(w, "| Cost/Purchase | %s |\n", m.CostPerPurchase)
	fmt.Fprintf(w, "| Purchase Value | %s |\n", m.PurchaseValue)
	fmt.Fprintf(w, "| ROAS | %s |\n", m.ROAS)
	fmt.Fprintf(w, "| Conversion Rate | %s |\n", m.ConversionRate)
	fmt.Fprintf(w, "| Engagement Rate | %s |\n", m.EngagementRate)
	fmt.Fprintf(w, "| Leads | %s |\n", m.Leads)
	fmt.Fprintf(w, "| Cost/Lead | %s |\n", m.CostPerLead)
}

// ── Progress output ──────────────────────────────────────────────────────────

func progress(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
