// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package sanctions

import (
	"encoding/csv"
	"encoding/xml"
	"io"
	"strings"

	"github.com/sudosoc/argus/internal/identity"
)

// Entry is one listed person or organisation under one name (the primary
// name or one alias).
type Entry struct {
	List     string // "OFAC SDN", "UN Security Council"
	ID       string
	Name     string // as listed, "First Last" order
	Alias    bool
	Kind     string // individual | entity
	Programs string
	Remarks  string
}

// flip turns OFAC's "SURNAME, Given Names" into "Given Names SURNAME".
func flip(name string) string {
	if a, b, ok := strings.Cut(name, ","); ok {
		return strings.TrimSpace(b) + " " + strings.TrimSpace(a)
	}
	return strings.TrimSpace(name)
}

// parseOFAC reads sdn.csv and (optionally) alt.csv. OFAC's files have no
// header row and use "-0- " for empty fields.
func parseOFAC(sdn, alt io.Reader) []Entry {
	clean := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "-0-" {
			return ""
		}
		return s
	}
	var out []Entry
	byID := map[string]*Entry{}
	r := csv.NewReader(sdn)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(rec) < 4 {
			continue
		}
		kind := "entity"
		if strings.EqualFold(clean(rec[2]), "individual") {
			kind = "individual"
		}
		e := Entry{List: "OFAC SDN", ID: clean(rec[0]), Name: flip(clean(rec[1])), Kind: kind, Programs: clean(rec[3])}
		if len(rec) > 11 {
			e.Remarks = clean(rec[11])
		}
		out = append(out, e)
		byID[e.ID] = &out[len(out)-1]
	}
	if alt != nil {
		ar := csv.NewReader(alt)
		ar.FieldsPerRecord = -1
		ar.LazyQuotes = true
		for {
			rec, err := ar.Read()
			if err == io.EOF {
				break
			}
			if err != nil || len(rec) < 4 {
				continue
			}
			base := byID[clean(rec[0])]
			if base == nil || clean(rec[3]) == "" {
				continue
			}
			out = append(out, Entry{List: base.List, ID: base.ID, Name: flip(clean(rec[3])), Alias: true, Kind: base.Kind, Programs: base.Programs, Remarks: base.Remarks})
		}
	}
	return out
}

type unPerson struct {
	Ref    string   `xml:"REFERENCE_NUMBER"`
	First  string   `xml:"FIRST_NAME"`
	Second string   `xml:"SECOND_NAME"`
	Third  string   `xml:"THIRD_NAME"`
	Fourth string   `xml:"FOURTH_NAME"`
	List   string   `xml:"UN_LIST_TYPE"`
	Alias  []string `xml:"INDIVIDUAL_ALIAS>ALIAS_NAME"`
	DOB    []string `xml:"INDIVIDUAL_DATE_OF_BIRTH>DATE"`
	Nat    []string `xml:"NATIONALITY>VALUE"`
}

type unEntity struct {
	Ref   string   `xml:"REFERENCE_NUMBER"`
	First string   `xml:"FIRST_NAME"`
	List  string   `xml:"UN_LIST_TYPE"`
	Alias []string `xml:"ENTITY_ALIAS>ALIAS_NAME"`
}

// parseUN reads the UN Security Council consolidated list.
func parseUN(r io.Reader) []Entry {
	var doc struct {
		People   []unPerson `xml:"INDIVIDUALS>INDIVIDUAL"`
		Entities []unEntity `xml:"ENTITIES>ENTITY"`
	}
	dec := xml.NewDecoder(r)
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil && len(doc.People)+len(doc.Entities) == 0 {
		return nil
	}
	var out []Entry
	for _, p := range doc.People {
		name := strings.Join(strings.Fields(strings.Join([]string{p.First, p.Second, p.Third, p.Fourth}, " ")), " ")
		rem := ""
		if len(p.DOB) > 0 {
			rem += "born " + strings.Join(p.DOB, ", ") + "; "
		}
		if len(p.Nat) > 0 {
			rem += "nationality " + strings.Join(p.Nat, ", ")
		}
		base := Entry{List: "UN Security Council", ID: strings.TrimSpace(p.Ref), Kind: "individual", Programs: strings.TrimSpace(p.List), Remarks: strings.TrimSpace(rem)}
		e := base
		e.Name = name
		out = append(out, e)
		for _, a := range p.Alias {
			if a = strings.TrimSpace(a); a != "" {
				al := base
				al.Name, al.Alias = a, true
				out = append(out, al)
			}
		}
	}
	for _, en := range doc.Entities {
		base := Entry{List: "UN Security Council", ID: strings.TrimSpace(en.Ref), Kind: "entity", Programs: strings.TrimSpace(en.List)}
		e := base
		e.Name = strings.TrimSpace(en.First)
		out = append(out, e)
		for _, a := range en.Alias {
			if a = strings.TrimSpace(a); a != "" {
				al := base
				al.Name, al.Alias = a, true
				out = append(out, al)
			}
		}
	}
	return out
}

// legalForms are dropped when comparing company names.
var legalForms = map[string]bool{"ltd": true, "limited": true, "llc": true, "inc": true, "incorporated": true, "corp": true, "corporation": true,
	"co": true, "company": true, "gmbh": true, "ag": true, "sa": true, "jsc": true, "ooo": true, "plc": true, "pty": true, "bv": true, "nv": true,
	"srl": true, "spa": true, "oy": true, "ab": true, "as": true, "kg": true, "fze": true, "fzc": true, "llp": true, "the": true, "and": true}

func orgWords(s string) []string {
	var out []string
	for _, w := range identity.Words(s) {
		if !legalForms[w] {
			out = append(out, w)
		}
	}
	return out
}

// index maps every word of every listed name to its entries so a query only
// compares against names that share a word with it.
type index struct {
	entries []Entry
	byWord  map[string][]int
}

func buildIndex(entries []Entry) *index {
	ix := &index{entries: entries, byWord: map[string][]int{}}
	for i, e := range entries {
		seen := map[string]bool{}
		words := identity.Words(e.Name)
		if e.Kind == "entity" {
			words = orgWords(e.Name)
		}
		for _, w := range words {
			if len(w) >= 3 && !seen[w] {
				seen[w] = true
				ix.byWord[w] = append(ix.byWord[w], i)
			}
		}
	}
	return ix
}

func (ix *index) candidates(words []string) []int {
	seen := map[int]bool{}
	var out []int
	for _, w := range words {
		for _, i := range ix.byWord[w] {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	return out
}

// matchPerson finds listed individuals whose name strongly matches name.
func (ix *index) matchPerson(name string) []Entry {
	if len(identity.Words(name)) < 2 {
		return nil
	}
	var out []Entry
	for _, i := range ix.candidates(identity.Words(name)) {
		e := ix.entries[i]
		if e.Kind == "individual" && len(identity.Words(e.Name)) >= 2 && identity.MatchNames(name, e.Name) == identity.NameStrong {
			out = append(out, e)
		}
	}
	return out
}

// matchOrg finds listed entities with the same significant words as name.
func (ix *index) matchOrg(name string) []Entry {
	want := orgWords(name)
	if len(want) == 0 || (len(want) == 1 && len(want[0]) < 5) {
		return nil
	}
	var out []Entry
	for _, i := range ix.candidates(want) {
		e := ix.entries[i]
		if e.Kind != "entity" {
			continue
		}
		got := orgWords(e.Name)
		if len(got) == len(want) && sameSet(got, want) {
			out = append(out, e)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}
