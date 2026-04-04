package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/billcobbler/golinks/internal/models"
)

// PrintLinks writes a table of links to w, or JSON if jsonMode is true.
func PrintLinks(w io.Writer, links []*models.Link, jsonMode bool) {
	if jsonMode {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(links)
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SHORTNAME\tURL\tCLICKS\tDESCRIPTION")
	fmt.Fprintln(tw, strings.Repeat("-", 9)+"\t"+strings.Repeat("-", 3)+"\t"+strings.Repeat("-", 6)+"\t"+strings.Repeat("-", 11))
	for _, l := range links {
		desc := l.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		target := l.TargetURL
		if len(target) > 55 {
			target = target[:52] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", l.Shortname, target, l.ClickCount, desc)
	}
	tw.Flush()
}

// PrintLink writes a single link's full details to w.
func PrintLink(w io.Writer, l *models.Link, jsonMode bool) {
	if jsonMode {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(l)
		return
	}
	fmt.Fprintf(w, "Shortname:   %s\n", l.Shortname)
	fmt.Fprintf(w, "URL:         %s\n", l.TargetURL)
	if l.Description != "" {
		fmt.Fprintf(w, "Description: %s\n", l.Description)
	}
	fmt.Fprintf(w, "Pattern:     %v\n", l.IsPattern)
	fmt.Fprintf(w, "Clicks:      %d\n", l.ClickCount)
	if l.LastClicked != nil {
		fmt.Fprintf(w, "Last click:  %s\n", l.LastClicked.Local().Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintf(w, "Created:     %s\n", l.CreatedAt.Local().Format("2006-01-02 15:04:05"))
}

// PrintStats writes aggregate statistics to w.
func PrintStats(w io.Writer, s *models.Stats, jsonMode bool) {
	if jsonMode {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(s)
		return
	}
	fmt.Fprintf(w, "Total links:  %d\n", s.TotalLinks)
	fmt.Fprintf(w, "Total clicks: %d\n", s.TotalClicks)
	if len(s.TopLinks) > 0 {
		fmt.Fprintln(w, "\nTop links:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, l := range s.TopLinks {
			fmt.Fprintf(tw, "  %s\t%d clicks\n", l.Shortname, l.ClickCount)
		}
		tw.Flush()
	}
}
