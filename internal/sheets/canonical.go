package sheets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// LoadCanonical accepts flat or {tables:{...}} registry/bootstrap JSON exports.
func LoadCanonical(path string) (map[string]string, map[string]bool, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, nil, e
	}
	if info.Size() > 64<<20 {
		return nil, nil, errors.New("canonical snapshot exceeds 64 MB")
	}
	var root map[string]json.RawMessage
	dec := json.NewDecoder(io.LimitReader(f, 64<<20))
	if e = dec.Decode(&root); e != nil {
		return nil, nil, e
	}
	if raw, ok := root["tables"]; ok {
		if e = json.Unmarshal(raw, &root); e != nil {
			return nil, nil, e
		}
	}
	convert := func(name string, heads []string) ([][]string, error) {
		raw, ok := root[name]
		if !ok {
			return nil, fmt.Errorf("canonical snapshot missing %s", name)
		}
		var records []map[string]json.RawMessage
		if e = json.Unmarshal(raw, &records); e != nil {
			return nil, e
		}
		rows := [][]string{heads}
		for _, r := range records {
			row := make([]string, len(heads))
			for i, h := range heads {
				v := r[h]
				if len(v) == 0 && h == "canonical_url" {
					v = r["url"]
				}
				if len(v) == 0 && (h == "account_id" && name == "accounts" || h == "persona_id" && name == "personas") {
					v = r["id"]
				}
				if len(v) > 0 && string(v) != "null" {
					if json.Unmarshal(v, &row[i]) != nil {
						row[i] = string(v)
					}
				}
			}
			rows = append(rows, row)
		}
		return rows, nil
	}
	accounts, e := convert("accounts", []string{"account_id", "platform", "platform_id", "canonical_url"})
	if e != nil {
		return nil, nil, e
	}
	links, e := convert("account_links", []string{"account_id", "persona_id", "review_status", "valid_to"})
	if e != nil {
		return nil, nil, e
	}
	personas, e := convert("personas", []string{"persona_id", "review_status"})
	if e != nil {
		return nil, nil, e
	}
	return CanonicalKeys(accounts, links, personas)
}
