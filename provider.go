package goneo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/libdns/libdns"
)

// Provider implements the libdns interfaces for Goneo and Caddy module.
type Provider struct {
	Host   string `json:"host,omitempty"`
	APIKey string `json:"api_key,omitempty"`
}

func init() {
	caddy.RegisterModule(Provider{})
}

// CaddyModule returns the Caddy module information.
func (Provider) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "dns.providers.goneo",
		New: func() caddy.Module { return &Provider{} },
	}
}

// Provision sets up the module.
func (p *Provider) Provision(ctx caddy.Context) error {
	repl := caddy.NewReplacer()
	p.Host = repl.ReplaceAll(p.Host, "")
	p.APIKey = repl.ReplaceAll(p.APIKey, "")
	
	if p.Host == "" {
		p.Host = "http://localhost:3000"
	}
	return nil
}

// UnmarshalCaddyfile sets up the DNS provider from Caddyfile tokens.
func (p *Provider) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			return d.ArgErr()
		}
		for nesting := d.Nesting(); d.NextBlock(nesting); {
			switch d.Val() {
			case "host":
				if d.NextArg() {
					p.Host = d.Val()
				} else {
					return d.ArgErr()
				}
			case "api_key":
				if d.NextArg() {
					p.APIKey = d.Val()
				} else {
					return d.ArgErr()
				}
			default:
				return d.Errf("unrecognized subdirective '%s'", d.Val())
			}
		}
	}
	return nil
}

type goneoRecord struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Target string `json:"target"`
	Prio   string `json:"prio"`
}

func (p *Provider) doRequest(ctx context.Context, method, endpoint string, body []byte) ([]byte, error) {
	url := strings.TrimRight(p.Host, "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	domain := strings.TrimRight(zone, ".")
	endpoint := fmt.Sprintf("/domains/%s/records", domain)
	
	resp, err := p.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []goneoRecord `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var records []libdns.Record
	for _, r := range result.Data {
		prio, _ := strconv.Atoi(r.Prio)
		records = append(records, libdns.Record{
			ID:       r.ID,
			Type:     r.Type,
			Name:     r.Name,
			Value:    r.Target,
			Priority: uint(prio),
		})
	}
	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	domain := strings.TrimRight(zone, ".")
	var appended []libdns.Record

	for _, rec := range records {
		endpoint := fmt.Sprintf("/domains/%s/records", domain)
		
		reqBody := map[string]interface{}{
			"type":    rec.Type,
			"name":    rec.Name,
			"content": rec.Value,
			"prio":    rec.Priority,
		}
		
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return appended, err
		}

		_, err = p.doRequest(ctx, "POST", endpoint, bodyBytes)
		if err != nil {
			return appended, err
		}
		
		appended = append(appended, rec)
	}
	return appended, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	domain := strings.TrimRight(zone, ".")
	var setRecs []libdns.Record

	for _, rec := range records {
		if rec.ID != "" {
			endpoint := fmt.Sprintf("/domains/%s/records/%s", domain, rec.ID)
			reqBody := map[string]interface{}{
				"type":    rec.Type,
				"name":    rec.Name,
				"content": rec.Value,
				"prio":    rec.Priority,
			}
			bodyBytes, _ := json.Marshal(reqBody)
			_, err := p.doRequest(ctx, "PUT", endpoint, bodyBytes)
			if err != nil {
				return setRecs, err
			}
			setRecs = append(setRecs, rec)
		} else {
			appended, err := p.AppendRecords(ctx, zone, []libdns.Record{rec})
			if err != nil {
				return setRecs, err
			}
			setRecs = append(setRecs, appended...)
		}
	}
	return setRecs, nil
}

// DeleteRecords deletes the records from the zone.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	domain := strings.TrimRight(zone, ".")
	var deleted []libdns.Record

	for _, rec := range records {
		id := rec.ID
		if id == "" {
			// Find ID by fetching records
			allRecs, err := p.GetRecords(ctx, zone)
			if err != nil {
				return deleted, err
			}
			for _, r := range allRecs {
				if r.Type == rec.Type && r.Name == rec.Name && r.Value == rec.Value {
					id = r.ID
					break
				}
			}
		}

		if id == "" {
			continue // Already deleted or not found
		}

		endpoint := fmt.Sprintf("/domains/%s/records/%s", domain, id)
		_, err := p.doRequest(ctx, "DELETE", endpoint, nil)
		if err != nil {
			return deleted, err
		}
		deleted = append(deleted, rec)
	}

	return deleted, nil
}

// Interface guards
var (
	_ caddyfile.Unmarshaler = (*Provider)(nil)
	_ caddy.Provisioner     = (*Provider)(nil)
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
