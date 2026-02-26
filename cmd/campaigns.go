package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ads-cli/internal/api"
	"github.com/the20100/meta-ads-cli/internal/output"
)

var (
	campaignStatusFilter string
	campaignLimit        int

	// create flags
	campaignName          string
	campaignObjective     string
	campaignDailyBudget   string
	campaignLifetimeBudget string
	campaignStatus        string

	// update flags
	campaignUpdateName           string
	campaignUpdateStatus         string
	campaignUpdateDailyBudget    string
	campaignUpdateLifetimeBudget string
)

var campaignsCmd = &cobra.Command{
	Use:   "campaigns",
	Short: "Manage Meta campaigns",
}

var campaignsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List campaigns for an ad account",
	RunE:  runCampaignsList,
}

var campaignsGetCmd = &cobra.Command{
	Use:   "get <campaign_id>",
	Short: "Get details for a campaign",
	Args:  cobra.ExactArgs(1),
	RunE:  runCampaignsGet,
}

var campaignsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new campaign",
	RunE:  runCampaignsCreate,
}

var campaignsPauseCmd = &cobra.Command{
	Use:   "pause <campaign_id>",
	Short: "Pause a campaign",
	Args:  cobra.ExactArgs(1),
	RunE:  runCampaignsPause,
}

var campaignsUpdateCmd = &cobra.Command{
	Use:   "update <campaign_id>",
	Short: "Update a campaign",
	Args:  cobra.ExactArgs(1),
	RunE:  runCampaignsUpdate,
}

func init() {
	// list flags
	campaignsListCmd.Flags().StringVar(&campaignStatusFilter, "status", "", "Filter by status (ACTIVE, PAUSED, ARCHIVED, etc.)")
	campaignsListCmd.Flags().IntVar(&campaignLimit, "limit", 0, "Max number of campaigns to return (0 = all)")

	// create flags
	campaignsCreateCmd.Flags().StringVar(&campaignName, "name", "", "Campaign name (required)")
	campaignsCreateCmd.Flags().StringVar(&campaignObjective, "objective", "", "Campaign objective e.g. OUTCOME_SALES, OUTCOME_AWARENESS (required)")
	campaignsCreateCmd.Flags().StringVar(&campaignDailyBudget, "daily-budget", "", "Daily budget in cents (e.g. 5000 = $50.00)")
	campaignsCreateCmd.Flags().StringVar(&campaignLifetimeBudget, "lifetime-budget", "", "Lifetime budget in cents")
	campaignsCreateCmd.Flags().StringVar(&campaignStatus, "status", "PAUSED", "Initial status (ACTIVE or PAUSED)")
	_ = campaignsCreateCmd.MarkFlagRequired("name")
	_ = campaignsCreateCmd.MarkFlagRequired("objective")

	// update flags
	campaignsUpdateCmd.Flags().StringVar(&campaignUpdateName, "name", "", "New campaign name")
	campaignsUpdateCmd.Flags().StringVar(&campaignUpdateStatus, "status", "", "New status (ACTIVE, PAUSED, ARCHIVED, DELETED)")
	campaignsUpdateCmd.Flags().StringVar(&campaignUpdateDailyBudget, "daily-budget", "", "New daily budget in cents")
	campaignsUpdateCmd.Flags().StringVar(&campaignUpdateLifetimeBudget, "lifetime-budget", "", "New lifetime budget in cents")

	campaignsCmd.AddCommand(campaignsListCmd, campaignsGetCmd, campaignsCreateCmd, campaignsPauseCmd, campaignsUpdateCmd)
	rootCmd.AddCommand(campaignsCmd)
}

func runCampaignsList(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	fields := "id,name,status,effective_status,objective,daily_budget,lifetime_budget,budget_remaining,bid_strategy,start_time,stop_time,created_time"
	params := url.Values{}
	params.Set("fields", fields)
	if campaignStatusFilter != "" {
		params.Set("effective_status", fmt.Sprintf(`["%s"]`, campaignStatusFilter))
	}
	if campaignLimit > 0 {
		params.Set("limit", fmt.Sprintf("%d", campaignLimit))
	}

	path := "/" + account + "/campaigns"

	var items []json.RawMessage
	if campaignLimit > 0 {
		// Fetch at most campaignLimit items (single page)
		params.Set("limit", fmt.Sprintf("%d", campaignLimit))
		body, err := client.Get(path, params)
		if err != nil {
			return err
		}
		var page struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
		items = page.Data
	} else {
		items, err = client.GetAll(path, params)
		if err != nil {
			return err
		}
	}

	campaigns := make([]api.Campaign, 0, len(items))
	for _, raw := range items {
		var c api.Campaign
		if err := json.Unmarshal(raw, &c); err != nil {
			return fmt.Errorf("parsing campaign: %w", err)
		}
		campaigns = append(campaigns, c)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(campaigns, prettyFlag)
	}

	headers := []string{"ID", "NAME", "STATUS", "OBJECTIVE", "DAILY BUDGET", "LIFETIME BUDGET"}
	rows := make([][]string, len(campaigns))
	for i, c := range campaigns {
		rows[i] = []string{
			c.ID,
			output.Truncate(c.Name, 45),
			c.EffectiveStatus,
			c.Objective,
			output.FormatBudget(c.DailyBudget),
			output.FormatBudget(c.LifetimeBudget),
		}
	}
	output.PrintTable(headers, rows)
	return nil
}

func runCampaignsGet(cmd *cobra.Command, args []string) error {
	id := args[0]
	fields := "id,name,status,effective_status,objective,daily_budget,lifetime_budget,budget_remaining,bid_strategy,start_time,stop_time,created_time,updated_time"
	params := url.Values{}
	params.Set("fields", fields)

	body, err := client.Get("/"+id, params)
	if err != nil {
		return err
	}

	var c api.Campaign
	if err := json.Unmarshal(body, &c); err != nil {
		return fmt.Errorf("parsing campaign: %w", err)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(c, prettyFlag)
	}

	rows := [][]string{
		{"ID", c.ID},
		{"Name", c.Name},
		{"Status", c.Status},
		{"Effective Status", c.EffectiveStatus},
		{"Objective", c.Objective},
		{"Daily Budget", output.FormatBudget(c.DailyBudget)},
		{"Lifetime Budget", output.FormatBudget(c.LifetimeBudget)},
		{"Budget Remaining", output.FormatBudget(c.BudgetRemaining)},
		{"Bid Strategy", c.BidStrategy},
		{"Start Time", c.StartTime},
		{"Stop Time", c.StopTime},
		{"Created", c.CreatedTime},
		{"Updated", c.UpdatedTime},
	}
	output.PrintKeyValue(rows)
	return nil
}

func runCampaignsCreate(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	body := url.Values{}
	body.Set("name", campaignName)
	body.Set("objective", campaignObjective)
	body.Set("status", campaignStatus)
	body.Set("special_ad_categories", "[]")
	if campaignDailyBudget != "" {
		body.Set("daily_budget", campaignDailyBudget)
	}
	if campaignLifetimeBudget != "" {
		body.Set("lifetime_budget", campaignLifetimeBudget)
	}

	resp, err := client.Post("/"+account+"/campaigns", body)
	if err != nil {
		return err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(result, prettyFlag)
	}
	fmt.Printf("✓ Campaign created: %s\n", result.ID)
	return nil
}

func runCampaignsPause(cmd *cobra.Command, args []string) error {
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
	fmt.Printf("✓ Campaign %s paused\n", id)
	return nil
}

func runCampaignsUpdate(cmd *cobra.Command, args []string) error {
	id := args[0]
	body := url.Values{}

	changed := false
	if campaignUpdateName != "" {
		body.Set("name", campaignUpdateName)
		changed = true
	}
	if campaignUpdateStatus != "" {
		body.Set("status", campaignUpdateStatus)
		changed = true
	}
	if campaignUpdateDailyBudget != "" {
		body.Set("daily_budget", campaignUpdateDailyBudget)
		changed = true
	}
	if campaignUpdateLifetimeBudget != "" {
		body.Set("lifetime_budget", campaignUpdateLifetimeBudget)
		changed = true
	}

	if !changed {
		return fmt.Errorf("no fields to update — use --name, --status, --daily-budget, or --lifetime-budget")
	}

	resp, err := client.Post("/"+id, body)
	if err != nil {
		return err
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(resp), prettyFlag)
	}
	fmt.Printf("✓ Campaign %s updated\n", id)
	return nil
}
