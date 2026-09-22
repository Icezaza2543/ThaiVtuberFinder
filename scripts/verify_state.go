package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sheets"
)

func main() {
	sheetID := os.Getenv("GOOGLE_SHEET_ID")
	if sheetID == "" {
		sheetID = "1mScOlcwCt8Ewh2f53_idCv-SYsFwcC9YfdoFeHs17E8"
	}
	net := netx.New()
	tokens, err := sheets.Credentials(net)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Credentials error: %v\n", err)
		os.Exit(1)
	}
	client := &sheets.Client{
		HTTP:          net,
		Tokens:        tokens,
		SpreadsheetID: sheetID,
	}

	ctx := context.Background()
	tables, err := client.Tables(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tables error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Canonical Baseline Verification ===")
	for _, name := range []string{"PERSONAS", "ACCOUNTS", "ACCOUNT_LINKS", "FINDER_INBOX"} {
		rows := tables[name]
		dataRows := 0
		if len(rows) > 0 {
			dataRows = len(rows) - 1
		}
		fmt.Printf("%s: total=%d (data_rows=%d)\n", name, len(rows), dataRows)
	}
}
