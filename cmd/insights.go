package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ads-cli/internal/output"
)

const defaultInsightFields = "impressions,clicks,spend,ctr,cpc,reach"

var (
	insightLevel      string
	insightSince      string
	insightUntil      string
	insightFields     string
	insightBreakdowns string
	insightLimit      int
)

var insightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Retrieve Meta Ads performance insights",
}

var insightsGetCmd = &cobra.Command{
	Use:   "get [object_id]",
	Short: "Get insights for an account, campaign, ad set, or ad",
	Long: `Get performance insights from the Meta Ads API.

By default, uses the account specified by --account.
Pass an explicit object ID (campaign, ad set, or ad) to get insights for that object.

Examples:
  # Account-level insights
  meta-ads insights get --account act_123 --since 2026-01-01 --until 2026-01-31

  # Campaign-level insights
  meta-ads insights get --account act_123 --level campaign --since 2026-01-01 --until 2026-01-31

  # Insights for a specific campaign
  meta-ads insights get 23851234567890 --since 2026-01-01 --until 2026-01-31

  # With custom fields and breakdowns
  meta-ads insights get --account act_123 --level ad --fields impressions,clicks,spend,ctr,cpc \
    --breakdowns age,gender --since 2026-01-01 --until 2026-01-31`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInsightsGet,
}

func init() {
	insightsGetCmd.Flags().StringVar(&insightLevel, "level", "account", "Aggregation level: account, campaign, adset, ad")
	insightsGetCmd.Flags().StringVar(&insightSince, "since", "", "Start date YYYY-MM-DD (required)")
	insightsGetCmd.Flags().StringVar(&insightUntil, "until", "", "End date YYYY-MM-DD (required)")
	insightsGetCmd.Flags().StringVar(&insightFields, "fields", defaultInsightFields, "Comma-separated insight fields")
	insightsGetCmd.Flags().StringVar(&insightBreakdowns, "breakdowns", "", "Comma-separated breakdowns (e.g. age,gender,country)")
	insightsGetCmd.Flags().IntVar(&insightLimit, "limit", 50, "Number of results per page")
	_ = insightsGetCmd.MarkFlagRequired("since")
	_ = insightsGetCmd.MarkFlagRequired("until")

	insightsCmd.AddCommand(insightsGetCmd)
	rootCmd.AddCommand(insightsCmd)
}

func runInsightsGet(cmd *cobra.Command, args []string) error {
	// Resolve the object ID: explicit arg or account
	var objectID string
	if len(args) == 1 {
		objectID = args[0]
	} else {
		account, err := resolveAccount()
		if err != nil {
			return err
		}
		objectID = account
	}

	fields := insightFields
	if fields == "" {
		fields = defaultInsightFields
	}

	// Add level-specific name fields for readable output
	nameFields := levelNameFields(insightLevel)
	if nameFields != "" {
		fields = nameFields + "," + fields
	}

	params := url.Values{}
	params.Set("fields", fields)
	params.Set("level", insightLevel)
	params.Set("time_range", fmt.Sprintf(`{"since":"%s","until":"%s"}`, insightSince, insightUntil))
	params.Set("limit", fmt.Sprintf("%d", insightLimit))

	if insightBreakdowns != "" {
		params.Set("breakdowns", insightBreakdowns)
	}

	items, err := client.GetAll("/"+objectID+"/insights", params)
	if err != nil {
		return err
	}

	if output.IsJSON(cmd) {
		// Output as parsed array
		result := make([]json.RawMessage, len(items))
		copy(result, items)
		return output.PrintJSON(result, prettyFlag)
	}

	// Build table from dynamic fields
	if len(items) == 0 {
		fmt.Println("No insights found for the specified period.")
		return nil
	}

	// Parse first item to determine columns
	var first map[string]json.RawMessage
	if err := json.Unmarshal(items[0], &first); err != nil {
		return fmt.Errorf("parsing insight: %w", err)
	}

	// Ordered columns: name fields first, then metric fields
	allFields := strings.Split(fields, ",")
	headers := make([]string, 0, len(allFields))
	for _, f := range allFields {
		f = strings.TrimSpace(f)
		if _, ok := first[f]; ok {
			headers = append(headers, strings.ToUpper(f))
		}
	}

	rows := make([][]string, 0, len(items))
	for _, raw := range items {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		row := make([]string, len(headers))
		for j, h := range headers {
			f := strings.ToLower(h)
			if v, ok := item[f]; ok {
				// Unquote JSON strings
				var s string
				if err := json.Unmarshal(v, &s); err == nil {
					row[j] = s
				} else {
					row[j] = string(v)
				}
			}
		}
		rows = append(rows, row)
	}

	output.PrintTable(headers, rows)
	return nil
}

// levelNameFields returns the identifying name fields for a given insight level.
func levelNameFields(level string) string {
	switch level {
	case "campaign":
		return "campaign_id,campaign_name"
	case "adset":
		return "adset_id,adset_name"
	case "ad":
		return "ad_id,ad_name"
	default:
		return "account_id,account_name"
	}
}
