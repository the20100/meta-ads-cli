package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ads-cli/internal/api"
	"github.com/the20100/meta-ads-cli/internal/output"
)

var audiencesCmd = &cobra.Command{
	Use:   "audiences",
	Short: "Manage Meta custom audiences",
}

var audiencesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List custom audiences for an ad account",
	RunE:  runAudiencesList,
}

func init() {
	audiencesCmd.AddCommand(audiencesListCmd)
	rootCmd.AddCommand(audiencesCmd)
}

func runAudiencesList(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	fields := "id,name,subtype,approximate_count_lower_bound,approximate_count_upper_bound,delivery_status,description,time_content_updated"
	params := url.Values{}
	params.Set("fields", fields)

	items, err := client.GetAll("/"+account+"/customaudiences", params)
	if err != nil {
		return err
	}

	audiences := make([]api.Audience, 0, len(items))
	for _, raw := range items {
		var a api.Audience
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing audience: %w", err)
		}
		audiences = append(audiences, a)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(audiences, prettyFlag)
	}

	headers := []string{"ID", "NAME", "SUBTYPE", "SIZE (LOW)", "SIZE (HIGH)", "STATUS"}
	rows := make([][]string, len(audiences))
	for i, a := range audiences {
		deliveryStatus := ""
		if a.DeliveryStatus != nil {
			deliveryStatus = a.DeliveryStatus.Description
		}
		rows[i] = []string{
			a.ID,
			output.Truncate(a.Name, 40),
			a.Subtype,
			formatCount(a.ApproximateCountLowerBound),
			formatCount(a.ApproximateCountUpperBound),
			output.Truncate(deliveryStatus, 30),
		}
	}
	output.PrintTable(headers, rows)
	return nil
}

func formatCount(n int) string {
	if n <= 0 {
		return "—"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
