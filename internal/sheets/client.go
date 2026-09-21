package sheets

import (
	"context"
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type Client struct {
	HTTP          *netx.Client
	Tokens        TokenSource
	SpreadsheetID string
	BaseURL       string
}

func (c *Client) api(ctx context.Context, method, path string, body, out any) error {
	if c.Tokens == nil {
		return ErrNotConfigured
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9_-]+$`, c.SpreadsheetID); !ok {
		return errors.New("invalid spreadsheet ID")
	}
	t, e := c.Tokens.Token(ctx)
	if e != nil {
		return e
	}
	base := c.BaseURL
	if base == "" {
		base = "https://sheets.googleapis.com/v4/spreadsheets/"
	}
	return c.HTTP.JSON(ctx, method, base+c.SpreadsheetID+path, map[string]string{"Authorization": "Bearer " + t}, body, out, nil)
}
func (c *Client) Tables(ctx context.Context) (map[string][][]string, error) {
	names := []string{"FINDER_INBOX", "ACCOUNTS", "ACCOUNT_LINKS", "PERSONAS"}
	ends := []string{"T", "N", "K", "L"}
	params := url.Values{"valueRenderOption": {"UNFORMATTED_VALUE"}}
	for i, n := range names {
		params.Add("ranges", "'"+n+"'!A:"+ends[i])
	}
	var doc struct {
		Ranges []struct {
			Values [][]any `json:"values"`
		} `json:"valueRanges"`
	}
	if e := c.api(ctx, "GET", "/values:batchGet?"+params.Encode(), nil, &doc); e != nil {
		return nil, e
	}
	if len(doc.Ranges) != len(names) {
		return nil, errors.New("incomplete spreadsheet batch read")
	}
	out := map[string][][]string{}
	for i, r := range doc.Ranges {
		rows := [][]string{}
		for _, r := range r.Values {
			line := make([]string, len(r))
			for k, v := range r {
				if v != nil {
					line[k] = fmt.Sprint(v)
				}
			}
			rows = append(rows, line)
		}
		out[names[i]] = rows
	}
	return out, nil
}
func (c *Client) ensureRows(ctx context.Context, needed int) error {
	var doc struct {
		Sheets []struct {
			Properties struct {
				ID    int    `json:"sheetId"`
				Title string `json:"title"`
				Grid  struct {
					Rows int `json:"rowCount"`
					Cols int `json:"columnCount"`
				} `json:"gridProperties"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if e := c.api(ctx, "GET", "?fields=sheets(properties(sheetId,title,gridProperties))", nil, &doc); e != nil {
		return e
	}
	cells := 0
	id := -1
	rows := 0
	cols := 0
	for _, s := range doc.Sheets {
		p := s.Properties
		cells += p.Grid.Rows * p.Grid.Cols
		if p.Title == "FINDER_INBOX" {
			id = p.ID
			rows = p.Grid.Rows
			cols = p.Grid.Cols
		}
	}
	if id < 0 || cols != 20 {
		return errors.New("FINDER_INBOX missing or not 20 columns")
	}
	if rows >= needed {
		return nil
	}
	target := ((needed + 999) / 1000) * 1000
	if cells+(target-rows)*cols > 8000000 {
		return errors.New("workbook safety budget of 8 million cells reached")
	}
	req := map[string]any{"requests": []any{map[string]any{"updateSheetProperties": map[string]any{"properties": map[string]any{"sheetId": id, "gridProperties": map[string]int{"rowCount": target}}, "fields": "gridProperties.rowCount"}}}}
	return c.api(ctx, "POST", ":batchUpdate", req, nil)
}

// Sync uses deterministic row-addressed updates; uncertain writes are safe to repeat.
// Run exactly one writer per workbook and do not sort/insert/delete rows during sync.
func (c *Client) Sync(ctx context.Context, candidates []model.Candidate) (int, error) {
	tables, e := c.Tables(ctx)
	if e != nil {
		return 0, e
	}
	known, _, e := CanonicalKeys(tables["ACCOUNTS"], tables["ACCOUNT_LINKS"], tables["PERSONAS"])
	if e != nil {
		return 0, e
	}
	plan, e := PlanSync(tables["FINDER_INBOX"], candidates, known)
	if e != nil {
		return 0, e
	}
	if len(plan) == 0 {
		return 0, nil
	}
	needed := len(tables["FINDER_INBOX"])
	re := regexp.MustCompile(`[A-Z]+([0-9]+)`)
	for _, u := range plan {
		for _, m := range re.FindAllStringSubmatch(u.Range, -1) {
			n, _ := strconv.Atoi(m[1])
			if n > needed {
				needed = n
			}
		}
	}
	if e = c.ensureRows(ctx, needed); e != nil {
		return 0, e
	}
	done := 0
	for start := 0; start < len(plan); start += 200 {
		if start > 0 {
			timer := time.NewTimer(1100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return done, ctx.Err()
			case <-timer.C:
			}
		}
		end := start + 200
		if end > len(plan) {
			end = len(plan)
		}
		body := map[string]any{"valueInputOption": "RAW", "data": plan[start:end]}
		if e = c.api(ctx, "POST", "/values:batchUpdate", body, nil); e != nil {
			return done, e
		}
		done += end - start
	}
	return done, nil
}
func (c *Client) Export(ctx context.Context) (Handoff, error) {
	tables, e := c.Tables(ctx)
	if e != nil {
		return Handoff{}, e
	}
	_, pids, e := CanonicalKeys(tables["ACCOUNTS"], tables["ACCOUNT_LINKS"], tables["PERSONAS"])
	if e != nil {
		return Handoff{}, e
	}
	return Proposals(tables["FINDER_INBOX"], pids)
}
