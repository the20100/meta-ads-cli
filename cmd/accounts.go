package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ads-cli/internal/api"
	"github.com/the20100/meta-ads-cli/internal/output"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Manage Meta Ad Accounts",
}

var accountsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all ad accounts accessible to you",
	RunE:  runAccountsList,
}

func init() {
	accountsCmd.AddCommand(accountsListCmd)
	rootCmd.AddCommand(accountsCmd)
}

func runAccountsList(cmd *cobra.Command, args []string) error {
	params := url.Values{}
	params.Set("fields", "id,name,currency,account_status,timezone_name,amount_spent,balance")

	items, err := client.GetAll("/me/adaccounts", params)
	if err != nil {
		return err
	}

	// Decode into Account structs
	accounts := make([]api.Account, 0, len(items))
	for _, raw := range items {
		var a api.Account
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing account: %w", err)
		}
		accounts = append(accounts, a)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(accounts, prettyFlag)
	}

	headers := []string{"ID", "NAME", "CURRENCY", "STATUS", "TIMEZONE", "AMOUNT SPENT", "BALANCE"}
	rows := make([][]string, len(accounts))
	for i, a := range accounts {
		rows[i] = []string{
			a.ID,
			output.Truncate(a.Name, 40),
			a.Currency,
			accountStatusLabel(a.Status),
			a.TimezoneName,
			output.FormatBudget(a.AmountSpent),
			output.FormatBudget(a.Balance),
		}
	}
	output.PrintTable(headers, rows)
	return nil
}

func accountStatusLabel(status int) string {
	switch status {
	case 1:
		return "ACTIVE"
	case 2:
		return "DISABLED"
	case 3:
		return "UNSETTLED"
	case 7:
		return "PENDING_RISK_REVIEW"
	case 8:
		return "PENDING_SETTLEMENT"
	case 9:
		return "IN_GRACE_PERIOD"
	case 100:
		return "PENDING_CLOSURE"
	case 101:
		return "CLOSED"
	case 201:
		return "ANY_ACTIVE"
	case 202:
		return "ANY_CLOSED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", status)
	}
}
