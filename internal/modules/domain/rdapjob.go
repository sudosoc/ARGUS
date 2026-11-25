// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/rdap"
	"github.com/sudosoc/argus/internal/util"
)

func rdapJob(ctx context.Context, c *httpx.Client, domain string, emit func(models.Finding)) {
	rec, err := rdap.LookupDomain(ctx, c, domain)
	if err != nil {
		if ctx.Err() == nil {
			emit(errFinding("WHOIS / RDAP", "whois", err))
		}
		return
	}
	d := map[string]string{"domain": rec.Name}
	if rec.Queried != domain {
		d["note"] = "record found for parent domain " + rec.Queried
	}
	util.Put(d, "registrar", rec.Registrar)
	util.Put(d, "created", rec.Created)
	util.Put(d, "expires", rec.Expires)
	util.Put(d, "last changed", rec.Updated)
	util.Put(d, "status", strings.Join(rec.Status, ", "))
	util.Put(d, "nameservers", strings.Join(rec.Nameservers, ", "))
	if rec.DNSSEC {
		d["DNSSEC"] = "signed"
	}
	if t, err := time.Parse(time.RFC3339, rec.Created); err == nil {
		d["age"] = humanAge(time.Since(t))
	}

	var ents []models.Entity
	seenEmail := map[string]bool{}
	for _, ct := range rec.Contacts {
		role := strings.Join(ct.Roles, "/")
		if role == "" {
			role = "contact"
		}
		if role == "registrar" {
			util.Put(d, "registrar contact email", ct.Email)
			continue
		}
		label := strings.TrimSpace(strings.Join(util.NonEmpty(ct.Name, ct.Org, ct.Email, ct.Phone), " · "))
		if label != "" {
			d[role] = label
		}
		if strings.Contains(ct.Email, "@") && !seenEmail[ct.Email] && !strings.Contains(role, "abuse") {
			seenEmail[ct.Email] = true
			ents = append(ents, models.Entity{Type: models.EntEmail, Value: ct.Email, Strong: true})
		}
		if ct.Name != "" && (strings.Contains(role, "registrant") || strings.Contains(role, "administrative")) {
			ents = append(ents, models.Entity{Type: models.EntName, Value: ct.Name})
		}
	}
	emit(fnd("WHOIS / RDAP registration", "whois", "https://rdap.org/domain/"+rec.Queried, 0.95, d, ents...))
}

func humanAge(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days < 60 {
		return strconv.Itoa(days) + " days"
	}
	if days < 730 {
		return strconv.Itoa(days/30) + " months"
	}
	return strconv.Itoa(days/365) + " years"
}
