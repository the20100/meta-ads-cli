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
	adAdsetFilter string
	adStatusFilter string
)

var adsCmd = &cobra.Command{
	Use:   "ads",
	Short: "Manage Meta ads",
}

var adsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ads for an ad account",
	RunE:  runAdsList,
}

var adsGetCmd = &cobra.Command{
	Use:   "get <ad_id>",
	Short: "Get details for an ad",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdsGet,
}

var adsPauseCmd = &cobra.Command{
	Use:   "pause <ad_id>",
	Short: "Pause an ad",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdsPause,
}

func init() {
	adsListCmd.Flags().StringVar(&adAdsetFilter, "adset", "", "Filter by ad set ID")
	adsListCmd.Flags().StringVar(&adStatusFilter, "status", "", "Filter by status (ACTIVE, PAUSED, etc.)")

	adsCmd.AddCommand(adsListCmd, adsGetCmd, adsPauseCmd)
	rootCmd.AddCommand(adsCmd)
}

func runAdsList(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	fields := "id,name,status,effective_status,adset_id,campaign_id,created_time,updated_time"
	params := url.Values{}
	params.Set("fields", fields)
	if adAdsetFilter != "" {
		params.Set("adset_id", adAdsetFilter)
	}
	if adStatusFilter != "" {
		params.Set("effective_status", fmt.Sprintf(`["%s"]`, adStatusFilter))
	}

	items, err := client.GetAll("/"+account+"/ads", params)
	if err != nil {
		return err
	}

	ads := make([]api.Ad, 0, len(items))
	for _, raw := range items {
		var a api.Ad
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing ad: %w", err)
		}
		ads = append(ads, a)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(ads, prettyFlag)
	}

	headers := []string{"ID", "NAME", "STATUS", "AD SET ID", "CAMPAIGN ID", "CREATED"}
	rows := make([][]string, len(ads))
	for i, a := range ads {
		rows[i] = []string{
			a.ID,
			output.Truncate(a.Name, 40),
			a.EffectiveStatus,
			a.AdSetID,
			a.CampaignID,
			output.FormatTime(a.CreatedTime),
		}
	}
	output.PrintTable(headers, rows)
	return nil
}

func runAdsGet(cmd *cobra.Command, args []string) error {
	id := args[0]
	fields := "id,name,status,effective_status,adset_id,campaign_id,creative,created_time,updated_time"
	params := url.Values{}
	params.Set("fields", fields)

	body, err := client.Get("/"+id, params)
	if err != nil {
		return err
	}

	var a api.Ad
	if err := json.Unmarshal(body, &a); err != nil {
		return fmt.Errorf("parsing ad: %w", err)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(a, prettyFlag)
	}

	rows := [][]string{
		{"ID", a.ID},
		{"Name", a.Name},
		{"Status", a.Status},
		{"Effective Status", a.EffectiveStatus},
		{"Ad Set ID", a.AdSetID},
		{"Campaign ID", a.CampaignID},
		{"Created", a.CreatedTime},
		{"Updated", a.UpdatedTime},
	}
	output.PrintKeyValue(rows)
	return nil
}

func runAdsPause(cmd *cobra.Command, args []string) error {
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
	fmt.Printf("✓ Ad %s paused\n", id)
	return nil
}
