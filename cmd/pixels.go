package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ads-cli/internal/api"
	"github.com/the20100/meta-ads-cli/internal/output"
)

var pixelsCmd = &cobra.Command{
	Use:   "pixels",
	Short: "Manage Meta pixels",
}

var pixelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pixels for an ad account",
	RunE:  runPixelsList,
}

func init() {
	pixelsCmd.AddCommand(pixelsListCmd)
	rootCmd.AddCommand(pixelsCmd)
}

func runPixelsList(cmd *cobra.Command, args []string) error {
	account, err := resolveAccount()
	if err != nil {
		return err
	}

	fields := "id,name,last_fired_time,creation_time,is_unavailable"
	params := url.Values{}
	params.Set("fields", fields)

	items, err := client.GetAll("/"+account+"/adspixels", params)
	if err != nil {
		return err
	}

	pixels := make([]api.Pixel, 0, len(items))
	for _, raw := range items {
		var p api.Pixel
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("parsing pixel: %w", err)
		}
		pixels = append(pixels, p)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(pixels, prettyFlag)
	}

	headers := []string{"ID", "NAME", "LAST FIRED", "CREATED", "UNAVAILABLE"}
	rows := make([][]string, len(pixels))
	for i, p := range pixels {
		unavailable := "no"
		if p.IsUnavailable {
			unavailable = "yes"
		}
		rows[i] = []string{
			p.ID,
			output.Truncate(p.Name, 40),
			output.FormatTime(p.LastFiredTime),
			output.FormatTime(p.CreationTime),
			unavailable,
		}
	}
	output.PrintTable(headers, rows)
	return nil
}
