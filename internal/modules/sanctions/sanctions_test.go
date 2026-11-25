// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package sanctions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

const sdnCSV = `1001,"PETROV, Ivan Sergeevich","individual","RUSSIA-EO14024","-0-","-0-","-0-","-0-","-0-","-0-","-0-","DOB 05 Mar 1970; nationality Russia"
1002,"ACME TRADING FZE","-0-","SDGT","-0-","-0-","-0-","-0-","-0-","-0-","-0-","-0-"
1003,"SMITH, John","individual","IRAN","-0-","-0-","-0-","-0-","-0-","-0-","-0-","-0-"
`
const altCSV = `1001,1,"aka","PETROV, Vanya","-0-"
1002,2,"aka","ACME TRADING LIMITED","-0-"
`
const unXML = `<?xml version="1.0"?><CONSOLIDATED_LIST><INDIVIDUALS>
<INDIVIDUAL><REFERENCE_NUMBER>QDi.001</REFERENCE_NUMBER><FIRST_NAME>Abdul</FIRST_NAME><SECOND_NAME>Rahman</SECOND_NAME><THIRD_NAME>Khan</THIRD_NAME><UN_LIST_TYPE>Al-Qaida</UN_LIST_TYPE>
<INDIVIDUAL_ALIAS><ALIAS_NAME>Abu Khan</ALIAS_NAME></INDIVIDUAL_ALIAS><NATIONALITY><VALUE>Afghanistan</VALUE></NATIONALITY></INDIVIDUAL>
</INDIVIDUALS><ENTITIES><ENTITY><REFERENCE_NUMBER>QDe.009</REFERENCE_NUMBER><FIRST_NAME>Northern Lights Foundation</FIRST_NAME><UN_LIST_TYPE>ISIL</UN_LIST_TYPE></ENTITY></ENTITIES></CONSOLIDATED_LIST>`

func TestParseOFAC(t *testing.T) {
	e := parseOFAC(strings.NewReader(sdnCSV), strings.NewReader(altCSV))
	if len(e) != 5 {
		t.Fatalf("%d entries: %+v", len(e), e)
	}
	if e[0].Name != "Ivan Sergeevich PETROV" || e[0].Kind != "individual" || !strings.Contains(e[0].Remarks, "1970") {
		t.Errorf("%+v", e[0])
	}
	if e[1].Kind != "entity" || e[1].Remarks != "" {
		t.Errorf("-0- must become empty: %+v", e[1])
	}
	if !e[3].Alias || e[3].Name != "Vanya PETROV" || e[3].ID != "1001" {
		t.Errorf("alias: %+v", e[3])
	}
}

func TestParseUN(t *testing.T) {
	e := parseUN(strings.NewReader(unXML))
	if len(e) != 3 || e[0].Name != "Abdul Rahman Khan" || !e[1].Alias || e[1].Name != "Abu Khan" || e[2].Kind != "entity" {
		t.Fatalf("%+v", e)
	}
	if !strings.Contains(e[0].Remarks, "Afghanistan") {
		t.Errorf("remarks: %q", e[0].Remarks)
	}
}

func TestMatching(t *testing.T) {
	ix := buildIndex(append(parseOFAC(strings.NewReader(sdnCSV), strings.NewReader(altCSV)), parseUN(strings.NewReader(unXML))...))
	if h := ix.matchPerson("Petrov Ivan"); len(h) == 0 {
		t.Error("order-insensitive person match")
	}
	if h := ix.matchPerson("Ivan Petrov"); len(h) == 0 {
		t.Error("person match")
	}
	if h := ix.matchPerson("Ivan Ivanov"); len(h) != 0 {
		t.Errorf("different surname must not match: %+v", h)
	}
	if h := ix.matchPerson("John"); h != nil {
		t.Error("single word names are too ambiguous to screen")
	}
	if h := ix.matchOrg("Acme Trading Ltd."); len(h) == 0 {
		t.Error("company match must ignore legal forms")
	}
	if h := ix.matchOrg("Acme Bakery"); len(h) != 0 {
		t.Error("different company")
	}
	if h := ix.matchOrg("Northern Lights Foundation"); len(h) == 0 {
		t.Error("UN entity")
	}
}

func TestScanEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sdn.csv":
			_, _ = w.Write([]byte(sdnCSV + strings.Repeat(" ", 100)))
		case "/alt.csv":
			_, _ = w.Write([]byte(altCSV + strings.Repeat(" ", 100)))
		case "/un.xml":
			_, _ = w.Write([]byte(unXML))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	oS, oA, oU := ofacSDN, ofacAlt, unList
	ofacSDN, ofacAlt, unList = []string{srv.URL + "/sdn.csv"}, []string{srv.URL + "/alt.csv"}, []string{srv.URL + "/un.xml"}
	defer func() { ofacSDN, ofacAlt, unList = oS, oA, oU }()

	c, _ := httpx.New(httpx.Options{Retries: 0})
	p := pool.New(2, 2)
	defer p.Close()
	var got []models.Finding
	s := New()
	err := s.Scan(context.Background(), models.Target{FullName: "Ivan Petrov", Company: "Totally Clean Ltd"}, scanner.Deps{HTTP: c, Pool: p}, func(f models.Finding) { got = append(got, f) })
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %+v", err, got)
	}
	if got[0].Status != models.StatusUncertain || !strings.Contains(got[0].Details["listed entries"], "PETROV") || got[0].Details["important"] == "" {
		t.Errorf("hit: %+v", got[0])
	}
	if got[1].Status != models.StatusNotFound || !strings.Contains(got[1].Details["lists checked"], "OFAC SDN (") {
		t.Errorf("clean company: %+v", got[1])
	}
	// second scan must come from the cache even if the server is gone
	srv.Close()
	got = nil
	_ = s.Scan(context.Background(), models.Target{FullName: "Abdul Rahman Khan"}, scanner.Deps{HTTP: c, Pool: p}, func(f models.Finding) { got = append(got, f) })
	if len(got) != 1 || got[0].Status != models.StatusUncertain {
		t.Errorf("cache: %+v", got)
	}
}

func TestScanReportsDownloadFailure(t *testing.T) {
	oS, oA, oU := ofacSDN, ofacAlt, unList
	ofacSDN, ofacAlt, unList = []string{"http://127.0.0.1:1/x"}, []string{"http://127.0.0.1:1/y"}, []string{"http://127.0.0.1:1/z"}
	defer func() { ofacSDN, ofacAlt, unList = oS, oA, oU }()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	var got []models.Finding
	_ = New().Scan(context.Background(), models.Target{FullName: "Ivan Petrov"}, scanner.Deps{HTTP: c}, func(f models.Finding) { got = append(got, f) })
	if len(got) != 2 || got[0].Status != models.StatusError {
		t.Errorf("coverage gaps must be reported as errors: %+v", got)
	}
}

func TestCanRun(t *testing.T) {
	s := New()
	if !s.CanRun(models.Target{FullName: "A B"}) || !s.CanRun(models.Target{Company: "X"}) || s.CanRun(models.Target{FullName: "Madonna"}) {
		t.Error("CanRun")
	}
}
