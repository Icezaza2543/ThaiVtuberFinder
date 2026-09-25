//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sheets"
)

type BackupPayload struct {
	Timestamp      string                   `json:"timestamp"`
	SpreadsheetID  string                   `json:"spreadsheet_id"`
	OriginalRows   int                      `json:"original_rows"`
	DuplicateCount int                      `json:"duplicate_count"`
	Groups         []sheets.DuplicateGroup `json:"duplicate_groups"`
	AllOriginal    [][]string               `json:"all_original_rows"`
}

func main() {
	applyFlag := flag.Bool("apply", false, "apply cleanup changes to Google Sheet (default is dry-run)")
	dryRunFlag := flag.Bool("dry-run", false, "explicitly run in dry-run mode")
	backupDir := flag.String("backup-dir", "data/backups", "directory to store JSON backup audit files")
	flag.Parse()

	isApply := *applyFlag && !*dryRunFlag

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
		fmt.Fprintf(os.Stderr, "Error fetching tables: %v\n", err)
		os.Exit(1)
	}

	inbox := tables["FINDER_INBOX"]
	fmt.Printf("Current FINDER_INBOX: %d rows (including header, %d data rows)\n", len(inbox), len(inbox)-1)

	groups, err := sheets.FindDuplicateInboxGroups(inbox)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding duplicate groups: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d duplicate account key group(s):\n", len(groups))
	for i, g := range groups {
		fmt.Printf("\nGroup %d: key=%q\n", i+1, g.Key)
		fmt.Printf("  SURVIVOR: SheetRow=%d, ID=%s, status=%s, action=%s, reviewer=%s, handle=%s, url=%s\n",
			g.Survivor.SheetRowNum, g.Survivor.Row[0], g.Survivor.Row[11], g.Survivor.Row[12], g.Survivor.Row[16], g.Survivor.Row[3], g.Survivor.Row[5])
		for _, l := range g.Losers {
			fmt.Printf("  LOSER:    SheetRow=%d, ID=%s, status=%s, action=%s, reviewer=%s, handle=%s, url=%s\n",
				l.SheetRowNum, l.Row[0], l.Row[11], l.Row[12], l.Row[16], l.Row[3], l.Row[5])
		}
	}

	if len(groups) == 0 {
		fmt.Println("\nNo duplicate groups found. Sheet is completely clean!")
		return
	}

	if !isApply {
		fmt.Println("\n[DRY RUN] No changes were made to Google Sheets.")
		fmt.Printf("To apply these changes, rerun with: --apply\n")
		return
	}

	// Apply mode:
	fmt.Printf("\n=== APPLYING CLEANUP (%d duplicate groups) ===\n", len(groups))

	// 1. Save backup audit file
	if err := os.MkdirAll(*backupDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create backup dir: %v\n", err)
		os.Exit(1)
	}
	ts := time.Now().UTC().Format("20060102-150405")
	backupPath := filepath.Join(*backupDir, fmt.Sprintf("inbox_backup_%s.json", ts))
	payload := BackupPayload{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		SpreadsheetID:  sheetID,
		OriginalRows:   len(inbox),
		DuplicateCount: len(groups),
		Groups:         groups,
		AllOriginal:    inbox,
	}
	bData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot marshal backup: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(backupPath, bData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write backup: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("1. Backup & audit trail saved: %s (%d bytes)\n", backupPath, len(bData))

	// 2. Prepare survivor updates
	var updates []sheets.Update
	for _, g := range groups {
		updates = append(updates, sheets.Update{
			Range:  fmt.Sprintf("'FINDER_INBOX'!A%d:T%d", g.Survivor.SheetRowNum, g.Survivor.SheetRowNum),
			Values: [][]string{g.Merged},
		})
	}
	fmt.Printf("2. Updating %d survivor row(s)...\n", len(updates))
	if err := client.UpdateInboxRows(ctx, updates); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating survivor rows: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Survivor rows updated successfully.\n")

	// 3. Delete loser rows
	var loserIndices []int
	for _, g := range groups {
		for _, l := range g.Losers {
			loserIndices = append(loserIndices, l.RowIndex)
		}
	}
	fmt.Printf("3. Deleting %d loser row(s)...\n", len(loserIndices))
	if err := client.DeleteInboxRows(ctx, loserIndices); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting loser rows: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Loser rows deleted successfully.\n")

	// 4. Verify post-cleanup state
	fmt.Println("4. Verifying post-cleanup state...")
	newTables, err := client.Tables(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error re-fetching tables: %v\n", err)
		os.Exit(1)
	}
	newInbox := newTables["FINDER_INBOX"]
	remainingGroups, err := sheets.FindDuplicateInboxGroups(newInbox)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking remaining groups: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nPost-cleanup verification:\n")
	fmt.Printf("  FINDER_INBOX data rows: %d (was %d, removed %d)\n", len(newInbox)-1, len(inbox)-1, len(loserIndices))
	fmt.Printf("  Duplicate normalized account keys remaining: %d\n", len(remainingGroups))

	if len(remainingGroups) != 0 {
		fmt.Fprintf(os.Stderr, "WARNING: Still found %d duplicate groups!\n", len(remainingGroups))
		os.Exit(1)
	}
	fmt.Println("SUCCESS: Exactly 0 duplicate account keys remain in FINDER_INBOX.")
}
