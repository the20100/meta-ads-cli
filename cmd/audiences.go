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

var audiencesCmd = &cobra.Command{
	Use:   "audiences",
	Short: "Manage Meta custom audiences",
}

var audiencesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List custom audiences for an ad account",
	RunE:  runAudiencesList,
}

var audiencesGetCmd = &cobra.Command{
	Use:   "get <audience_id>",
	Short: "Get details for a custom audience (including construction rules)",
	Args:  cobra.ExactArgs(1),
	RunE:  runAudiencesGet,
}

func init() {
	audiencesCmd.AddCommand(audiencesListCmd, audiencesGetCmd)
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

func runAudiencesGet(cmd *cobra.Command, args []string) error {
	id := args[0]
	fields := "id,name,description,subtype,rule,rule_aggregation,retention_days,pixel_id,approximate_count_lower_bound,approximate_count_upper_bound,delivery_status,time_created,time_updated,time_content_updated"
	params := url.Values{}
	params.Set("fields", fields)

	body, err := client.Get("/"+id, params)
	if err != nil {
		return err
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(body), prettyFlag)
	}

	var a api.Audience
	if err := json.Unmarshal(body, &a); err != nil {
		return fmt.Errorf("parsing audience: %w", err)
	}

	deliveryStatus := ""
	if a.DeliveryStatus != nil {
		deliveryStatus = a.DeliveryStatus.Description
	}

	rows := [][]string{
		{"ID", a.ID},
		{"Name", a.Name},
		{"Subtype", a.Subtype},
		{"Description", a.Description},
		{"Pixel ID", a.PixelID},
		{"Retention Days", formatRetention(a.RetentionDays)},
		{"Size (Lower)", formatCount(a.ApproximateCountLowerBound)},
		{"Size (Upper)", formatCount(a.ApproximateCountUpperBound)},
		{"Delivery Status", deliveryStatus},
		{"Created", output.FormatTime(a.TimeCreated)},
		{"Updated", output.FormatTime(a.TimeUpdated)},
		{"Content Updated", output.FormatTime(a.TimeContentUpdated)},
	}
	output.PrintKeyValue(rows)

	// Display rule details
	if len(a.Rule) > 0 {
		fmt.Println()
		fmt.Println("CONSTRUCTION RULES")
		fmt.Println(strings.Repeat("─", 60))
		printAudienceRules(a.Rule)
	}

	return nil
}

// printAudienceRules parses and displays audience construction rules.
func printAudienceRules(raw json.RawMessage) {
	// The rule field can be a JSON string (stringified JSON) or a JSON object
	var ruleStr string
	if err := json.Unmarshal(raw, &ruleStr); err == nil {
		// It's a stringified JSON — parse the inner JSON
		raw = json.RawMessage(ruleStr)
	}

	var rule map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rule); err != nil {
		// Fallback: print raw
		printAudienceIndentedJSON(raw)
		return
	}

	// Parse inclusions
	if incl, ok := rule["inclusions"]; ok {
		fmt.Println("  Inclusions:")
		printRuleGroup(incl, "    ")
	}

	// Parse exclusions
	if excl, ok := rule["exclusions"]; ok {
		fmt.Println("  Exclusions:")
		printRuleGroup(excl, "    ")
	}

	// If neither inclusions nor exclusions, show the whole rule
	if _, hasIncl := rule["inclusions"]; !hasIncl {
		if _, hasExcl := rule["exclusions"]; !hasExcl {
			printAudienceIndentedJSON(raw)
		}
	}
}

// printRuleGroup prints an inclusion/exclusion group.
func printRuleGroup(raw json.RawMessage, indent string) {
	var group struct {
		Operator string `json:"operator"`
		Rules    []struct {
			EventSources []struct {
				Type string `json:"type"`
				ID   json.Number `json:"id"`
			} `json:"event_sources"`
			RetentionSeconds int `json:"retention_seconds"`
			Filter           *struct {
				Operator string `json:"operator"`
				Filters  []struct {
					Field    string `json:"field"`
					Operator string `json:"operator"`
					Value    string `json:"value"`
				} `json:"filters"`
			} `json:"filter"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &group); err != nil {
		printAudienceIndentedJSON(raw)
		return
	}

	fmt.Printf("%sOperator: %s\n", indent, group.Operator)
	for i, r := range group.Rules {
		fmt.Printf("%sRule %d:\n", indent, i+1)
		// Event sources
		for _, es := range r.EventSources {
			fmt.Printf("%s  Source: %s (ID: %s)\n", indent, es.Type, es.ID.String())
		}
		// Retention
		if r.RetentionSeconds > 0 {
			days := r.RetentionSeconds / 86400
			fmt.Printf("%s  Retention: %d days\n", indent, days)
		}
		// Filters
		if r.Filter != nil && len(r.Filter.Filters) > 0 {
			fmt.Printf("%s  Filters (%s):\n", indent, r.Filter.Operator)
			for _, f := range r.Filter.Filters {
				fmt.Printf("%s    %s %s %s\n", indent, f.Field, f.Operator, f.Value)
			}
		}
	}
}

func printAudienceIndentedJSON(raw json.RawMessage) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Printf("  %s\n", string(raw))
		return
	}
	b, _ := json.MarshalIndent(v, "  ", "  ")
	fmt.Printf("  %s\n", string(b))
}

func formatRetention(days int) string {
	if days <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", days)
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
