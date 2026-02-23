package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ads-cli/internal/api"
	"github.com/vincentmaurin/meta-ads-cli/internal/output"
)

var (
	adsetCampaignFilter string
	adsetStatusFilter   string

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
	for _, raw := range items {
		var a api.AdSet
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing adset: %w", err)
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
			output.FormatBudget(a.DailyBudget),
			a.BillingEvent,
			a.OptimizationGoal,
		}
	}
	output.PrintTable(headers, rows)
	return nil
}

func runAdsetsGet(cmd *cobra.Command, args []string) error {
	id := args[0]
	fields := "id,name,status,effective_status,campaign_id,daily_budget,lifetime_budget,budget_remaining,bid_amount,billing_event,optimization_goal,start_time,end_time,created_time,updated_time"
	params := url.Values{}
	params.Set("fields", fields)

	body, err := client.Get("/"+id, params)
	if err != nil {
		return err
	}

	var a api.AdSet
	if err := json.Unmarshal(body, &a); err != nil {
		return fmt.Errorf("parsing adset: %w", err)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(a, prettyFlag)
	}

	rows := [][]string{
		{"ID", a.ID},
		{"Name", a.Name},
		{"Status", a.Status},
		{"Effective Status", a.EffectiveStatus},
		{"Campaign ID", a.CampaignID},
		{"Daily Budget", output.FormatBudget(a.DailyBudget)},
		{"Lifetime Budget", output.FormatBudget(a.LifetimeBudget)},
		{"Budget Remaining", output.FormatBudget(a.BudgetRemaining)},
		{"Bid Amount", a.BidAmount},
		{"Billing Event", a.BillingEvent},
		{"Optimization Goal", a.OptimizationGoal},
		{"Start Time", a.StartTime},
		{"End Time", a.EndTime},
		{"Created", a.CreatedTime},
		{"Updated", a.UpdatedTime},
	}
	output.PrintKeyValue(rows)
	return nil
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
