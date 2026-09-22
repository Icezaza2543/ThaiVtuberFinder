package sheets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
)

type DuplicateEntry struct {
	RowIndex    int      // 0-indexed in table (row 1 is header, row index 1 is sheet row 2)
	SheetRowNum int      // 1-indexed sheet row number (i.e. RowIndex + 1)
	Row         []string // 20 padded columns
}

type DuplicateGroup struct {
	Key      string
	Survivor DuplicateEntry
	Losers   []DuplicateEntry
	Merged   []string
}

// ScoreRow calculates survivor priority:
// 1. review_status == "verified" (10,000 pts)
// 2. has reviewer or reviewed_at (1,000 pts)
// 3. has target_persona_id (100 pts)
// 4. has non-empty action (10 pts)
// 5. earlier row (tie breaker via row index)
func ScoreRow(r []string, rowIndex int) int {
	score := 0
	row := padded(r)
	if row[11] == "verified" {
		score += 10000
	} else if row[11] != "" && row[11] != "pending" {
		score += 5000
	}
	if row[16] != "" || row[17] != "" {
		score += 1000
	}
	if row[13] != "" {
		score += 100
	}
	if row[12] != "" {
		score += 10
	}
	// Prefer earlier row index as tie-breaker
	score += (100000 - rowIndex)
	return score
}

func FindDuplicateInboxGroups(rows [][]string) ([]DuplicateGroup, error) {
	if e := checkInbox(rows); e != nil {
		return nil, e
	}
	byKey := map[string][]DuplicateEntry{}
	for i, r := range rows[1:] {
		row := padded(r)
		if row[0] == "" {
			continue
		}
		a := RowAccount(r)
		if a.Platform != "" && a.URL != "" {
			k := a.Key()
			byKey[k] = append(byKey[k], DuplicateEntry{
				RowIndex:    i + 1,
				SheetRowNum: i + 2,
				Row:         row,
			})
		}
	}

	var groups []DuplicateGroup
	for k, entries := range byKey {
		if len(entries) <= 1 {
			continue
		}
		// Sort entries by score descending
		sort.Slice(entries, func(i, j int) bool {
			return ScoreRow(entries[i].Row, entries[i].RowIndex) > ScoreRow(entries[j].Row, entries[j].RowIndex)
		})

		survivor := entries[0]
		losers := entries[1:]
		merged := make([]string, 20)
		copy(merged, survivor.Row)

		// Merge machine metadata from losers into survivor
		for _, loser := range losers {
			lRow := loser.Row
			// Fill missing machine fields if survivor is empty
			if merged[2] == "" && lRow[2] != "" {
				merged[2] = lRow[2] // platform_id
			}
			if merged[3] == "" && lRow[3] != "" {
				merged[3] = lRow[3] // handle
			}
			if merged[4] == "" && lRow[4] != "" {
				merged[4] = lRow[4] // display_name
			}
			// Normalize canonical_url
			if norm, err := model.Normalize(merged[5]); err == nil {
				merged[5] = norm.URL
			}
			if merged[6] == "UNRESOLVED" && lRow[6] != "UNRESOLVED" && lRow[6] != "" {
				merged[6] = lRow[6]
			}
			if merged[7] == "uncertain" && lRow[7] != "uncertain" && lRow[7] != "" {
				merged[7] = lRow[7]
			}
			// Earliest first_seen
			sTime, sErr := time.Parse(time.RFC3339, merged[9])
			lTime, lErr := time.Parse(time.RFC3339, lRow[9])
			if sErr == nil && lErr == nil && lTime.Before(sTime) {
				merged[9] = lRow[9]
			}
			// Latest last_checked
			sCheck, sCErr := time.Parse(time.RFC3339, merged[10])
			lCheck, lCErr := time.Parse(time.RFC3339, lRow[10])
			if sCErr == nil && lCErr == nil && lCheck.After(sCheck) {
				merged[10] = lRow[10]
			}

			// Preserve curator fields: if survivor empty and loser has it
			for col := 11; col <= 17; col++ {
				if merged[col] == "" && lRow[col] != "" {
					merged[col] = lRow[col]
				}
			}
			// Notes: append merge audit note if loser had notes
			if lRow[18] != "" && !strings.Contains(merged[18], lRow[0]) {
				if merged[18] != "" {
					merged[18] += "; "
				}
				merged[18] += fmt.Sprintf("merged_duplicate:%s", lRow[0])
			}
		}

		groups = append(groups, DuplicateGroup{
			Key:      k,
			Survivor: survivor,
			Losers:   losers,
			Merged:   merged,
		})
	}

	// Sort groups deterministically by survivor sheet row number
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Survivor.SheetRowNum < groups[j].Survivor.SheetRowNum
	})

	return groups, nil
}

// DeleteInboxRows deletes specified 0-indexed row indices in FINDER_INBOX from Google Sheets.
// Indices are deleted in descending order.
func (c *Client) DeleteInboxRows(ctx context.Context, rowIndices []int) error {
	if len(rowIndices) == 0 {
		return nil
	}
	var doc struct {
		Sheets []struct {
			Properties struct {
				ID    int    `json:"sheetId"`
				Title string `json:"title"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if e := c.api(ctx, "GET", "?fields=sheets(properties(sheetId,title))", nil, &doc); e != nil {
		return e
	}
	inboxID := -1
	for _, s := range doc.Sheets {
		if s.Properties.Title == "FINDER_INBOX" {
			inboxID = s.Properties.ID
			break
		}
	}
	if inboxID < 0 {
		return errors.New("FINDER_INBOX sheet ID not found")
	}

	// Sort row indices descending so deleting does not change indices of earlier rows
	sorted := append([]int{}, rowIndices...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] > sorted[j] })

	requests := make([]any, len(sorted))
	for i, idx := range sorted {
		requests[i] = map[string]any{
			"deleteDimension": map[string]any{
				"range": map[string]any{
					"sheetId":    inboxID,
					"dimension":  "ROWS",
					"startIndex": idx,
					"endIndex":   idx + 1,
				},
			},
		}
	}

	body := map[string]any{"requests": requests}
	return c.api(ctx, "POST", ":batchUpdate", body, nil)
}

// UpdateInboxRows writes full row values (A:T) for specified survivor rows.
func (c *Client) UpdateInboxRows(ctx context.Context, updates []Update) error {
	if len(updates) == 0 {
		return nil
	}
	data := make([]map[string]any, len(updates))
	for i, u := range updates {
		data[i] = map[string]any{
			"range":  u.Range,
			"values": u.Values,
		}
	}
	body := map[string]any{
		"valueInputOption": "RAW",
		"data":             data,
	}
	return c.api(ctx, "POST", "/values:batchUpdate", body, nil)
}
