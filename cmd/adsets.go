package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ads-cli/internal/api"
	"github.com/the20100/meta-ads-cli/internal/output"
)

var (
	adsetCampaignFilter    string
	adsetStatusFilter      string
	adsetNameContains      string
	adsetGetFields         string

	adsetUpdateDailyBudget    string
	adsetUpdateLifetimeBudget string
)

var adsetsCmd = &cobra.Command{
	Use:   "adsets",
	Short: "Manage Meta ad sets",
}

var adsetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ad sets for an ad account",
	RunE:  runAdsetsList,
}

var adsetsGetCmd = &cobra.Command{
	Use:   "get <adset_id>",
	Short: "Get details for an ad set",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdsetsGet,
}

var adsetsPauseCmd = &cobra.Command{
	Use:   "pause <adset_id>",
	Short: "Pause an ad set",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdsetsPause,
}

var adsetsUpdateBudgetCmd = &cobra.Command{
	Use:   "update-budget <adset_id>",
	Short: "Update the budget for an ad set",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdsetsUpdateBudget,
}

func init() {
	adsetsListCmd.Flags().StringVar(&adsetCampaignFilter, "campaign", "", "Filter by campaign ID")
	adsetsListCmd.Flags().StringVar(&adsetStatusFilter, "status", "", "Filter by status (ACTIVE, PAUSED, etc.)")
	adsetsListCmd.Flags().StringVar(&adsetNameContains, "name-contains", "", "Filter ad sets whose name contains this string (case-insensitive)")

	adsetsGetCmd.Flags().StringVar(&adsetGetFields, "fields", "", "Comma-separated fields to request from the API (overrides defaults)")

	adsetsUpdateBudgetCmd.Flags().StringVar(&adsetUpdateDailyBudget, "daily-budget", "", "New daily budget in cents (e.g. 5000 = $50.00)")
	adsetsUpdateBudgetCmd.Flags().StringVar(&adsetUpdateLifetimeBudget, "lifetime-budget", "", "New lifetime budget in cents")

	adsetsCmd.AddCommand(adsetsListCmd, adsetsGetCmd, adsetsPauseCmd, adsetsUpdateBudgetCmd)
	rootCmd.AddCommand(adsetsCmd)
}

func runAdsetsList(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	fields := "id,name,status,effective_status,campaign_id,daily_budget,lifetime_budget,budget_remaining,bid_amount,billing_event,optimization_goal,start_time,end_time,created_time"
	params := url.Values{}
	params.Set("fields", fields)
	if adsetCampaignFilter != "" {
		params.Set("campaign_id", adsetCampaignFilter)
	}
	if adsetStatusFilter != "" {
		params.Set("effective_status", fmt.Sprintf(`["%s"]`, adsetStatusFilter))
	}

	items, err := client.GetAll("/"+account+"/adsets", params)
	if err != nil {
		return err
	}

	adsets := make([]api.AdSet, 0, len(items))
	nameFilter := strings.ToLower(adsetNameContains)
	for _, raw := range items {
		var a api.AdSet
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing adset: %w", err)
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(a.Name), nameFilter) {
			continue
		}
		adsets = append(adsets, a)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(adsets, prettyFlag)
	}

	headers := []string{"ID", "NAME", "STATUS", "CAMPAIGN ID", "DAILY BUDGET", "BILLING EVENT", "OPT. GOAL"}
	rows := make([][]string, len(adsets))
	for i, a := range adsets {
		rows[i] = []string{
			a.ID,
			output.Truncate(a.Name, 38),
			a.EffectiveStatus,
			a.CampaignID,
			output.FormatBudget(a.DailyBudget.String()),
			a.BillingEvent,
			a.OptimizationGoal,
		}
	}
	output.PrintTable(headers, rows)
	return nil
}

func runAdsetsGet(cmd *cobra.Command, args []string) error {
	id := args[0]
	fields := "id,name,status,effective_status,campaign_id,daily_budget,lifetime_budget,budget_remaining,bid_amount,bid_strategy,billing_event,optimization_goal,start_time,end_time,created_time,updated_time,destination_type,campaign{id,name,objective},targeting,promoted_object,attribution_spec,pacing_type"
	if adsetGetFields != "" {
		fields = adsetGetFields
	}
	params := url.Values{}
	params.Set("fields", fields)

	body, err := client.Get("/"+id, params)
	if err != nil {
		return err
	}

	if output.IsJSON(cmd) {
		// For JSON output, return the raw response to preserve all nested structures
		return output.PrintJSON(json.RawMessage(body), prettyFlag)
	}

	var a api.AdSet
	if err := json.Unmarshal(body, &a); err != nil {
		return fmt.Errorf("parsing adset: %w", err)
	}

	campaignInfo := a.CampaignID
	if a.Campaign != nil {
		campaignInfo = fmt.Sprintf("%s (%s) — %s", a.Campaign.Name, a.Campaign.ID, a.Campaign.Objective)
	}

	rows := [][]string{
		{"ID", a.ID},
		{"Name", a.Name},
		{"Status", a.Status},
		{"Effective Status", a.EffectiveStatus},
		{"Campaign", campaignInfo},
		{"Daily Budget", output.FormatBudget(a.DailyBudget.String())},
		{"Lifetime Budget", output.FormatBudget(a.LifetimeBudget.String())},
		{"Budget Remaining", output.FormatBudget(a.BudgetRemaining.String())},
		{"Bid Amount", a.BidAmount.String()},
		{"Bid Strategy", a.BidStrategy},
		{"Billing Event", a.BillingEvent},
		{"Optimization Goal", a.OptimizationGoal},
		{"Destination Type", a.DestinationType},
		{"Start Time", output.FormatTime(a.StartTime)},
		{"End Time", output.FormatTime(a.EndTime)},
		{"Created", output.FormatTime(a.CreatedTime)},
		{"Updated", output.FormatTime(a.UpdatedTime)},
	}
	output.PrintKeyValue(rows)

	// Display targeting summary
	if len(a.Targeting) > 0 {
		fmt.Println()
		fmt.Println("TARGETING")
		fmt.Println(strings.Repeat("─", 60))
		printTargetingSummary(a.Targeting)
	}

	// Display promoted object
	if len(a.PromotedObject) > 0 {
		fmt.Println()
		fmt.Println("PROMOTED OBJECT")
		fmt.Println(strings.Repeat("─", 60))
		printIndentedJSON(a.PromotedObject)
	}

	// Display attribution spec
	if len(a.AttributionSpec) > 0 {
		fmt.Println()
		fmt.Println("ATTRIBUTION SPEC")
		fmt.Println(strings.Repeat("─", 60))
		printIndentedJSON(a.AttributionSpec)
	}

	return nil
}

// printTargetingSummary prints key targeting fields in a readable format.
func printTargetingSummary(raw json.RawMessage) {
	var targeting map[string]json.RawMessage
	if err := json.Unmarshal(raw, &targeting); err != nil {
		printIndentedJSON(raw)
		return
	}

	// Age & gender
	if v, ok := targeting["age_min"]; ok {
		fmt.Printf("  Age Min:         %s\n", string(v))
	}
	if v, ok := targeting["age_max"]; ok {
		fmt.Printf("  Age Max:         %s\n", string(v))
	}
	if v, ok := targeting["genders"]; ok {
		fmt.Printf("  Genders:         %s\n", string(v))
	}

	// Geo
	if v, ok := targeting["geo_locations"]; ok {
		var geo map[string]json.RawMessage
		if err := json.Unmarshal(v, &geo); err == nil {
			if countries, ok := geo["countries"]; ok {
				fmt.Printf("  Countries:       %s\n", string(countries))
			}
			if lt, ok := geo["location_types"]; ok {
				fmt.Printf("  Location Types:  %s\n", string(lt))
			}
		}
	}

	// Publisher platforms
	if v, ok := targeting["publisher_platforms"]; ok {
		fmt.Printf("  Platforms:       %s\n", string(v))
	}
	if v, ok := targeting["facebook_positions"]; ok {
		fmt.Printf("  FB Positions:    %s\n", string(v))
	}
	if v, ok := targeting["instagram_positions"]; ok {
		fmt.Printf("  IG Positions:    %s\n", string(v))
	}

	// Custom audiences
	if v, ok := targeting["custom_audiences"]; ok {
		var audiences []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(v, &audiences); err == nil && len(audiences) > 0 {
			fmt.Println("  Included Audiences:")
			for _, a := range audiences {
				name := a.ID
				if a.Name != "" {
					name = fmt.Sprintf("%s (%s)", a.Name, a.ID)
				}
				fmt.Printf("    + %s\n", name)
			}
		}
	}
	if v, ok := targeting["excluded_custom_audiences"]; ok {
		var audiences []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(v, &audiences); err == nil && len(audiences) > 0 {
			fmt.Println("  Excluded Audiences:")
			for _, a := range audiences {
				name := a.ID
				if a.Name != "" {
					name = fmt.Sprintf("%s (%s)", a.Name, a.ID)
				}
				fmt.Printf("    - %s\n", name)
			}
		}
	}

	// Flexible spec (interests, behaviors, etc.)
	if v, ok := targeting["flexible_spec"]; ok {
		fmt.Printf("  Flexible Spec:   (use --json for details)\n")
		_ = v
	}
}

// printIndentedJSON prints raw JSON with indentation.
func printIndentedJSON(raw json.RawMessage) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Printf("  %s\n", string(raw))
		return
	}
	b, _ := json.MarshalIndent(v, "  ", "  ")
	fmt.Printf("  %s\n", string(b))
}

func runAdsetsPause(cmd *cobra.Command, args []string) error {
	id := args[0]
	body := url.Values{}
	body.Set("status", "PAUSED")

	resp, err := client.Post("/"+id, body)
	if err != nil {
		return err
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(resp), prettyFlag)
	}
	fmt.Printf("✓ Ad set %s paused\n", id)
	return nil
}

func runAdsetsUpdateBudget(cmd *cobra.Command, args []string) error {
	id := args[0]
	body := url.Values{}

	changed := false
	if adsetUpdateDailyBudget != "" {
		body.Set("daily_budget", adsetUpdateDailyBudget)
		changed = true
	}
	if adsetUpdateLifetimeBudget != "" {
		body.Set("lifetime_budget", adsetUpdateLifetimeBudget)
		changed = true
	}

	if !changed {
		return fmt.Errorf("no budget specified — use --daily-budget or --lifetime-budget")
	}

	resp, err := client.Post("/"+id, body)
	if err != nil {
		return err
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(resp), prettyFlag)
	}
	fmt.Printf("✓ Ad set %s budget updated\n", id)
	return nil
}
