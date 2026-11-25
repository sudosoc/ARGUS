package rdap

import (
	"encoding/json"
	"github.com/sudosoc/argus/internal/util"
	"testing"
)

func TestRedacted(t *testing.T) {
	for _, v := range []string{"", "REDACTED FOR PRIVACY", "Data Protected", "not disclosed", "Privacy Contact"} {
		if !redacted(v) {
			t.Errorf("%q should be redacted", v)
		}
	}
	for _, v := range []string{"Jane Doe", "Acme Inc", "abuse@acme.org"} {
		if redacted(v) {
			t.Errorf("%q is real data", v)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if util.FirstNonEmpty("", "", "x", "y") != "x" || util.FirstNonEmpty() != "" {
		t.Error("firstNonEmpty")
	}
}

func TestFlatten(t *testing.T) {
	raw := `{"roles":["registrant"],"vcardArray":["vcard",[
		["version",{},"text","4.0"],
		["fn",{},"text","Jane Doe"],
		["org",{},"text",["Acme Inc"]],
		["email",{},"text","Jane@Acme.ORG"],
		["tel",{},"uri","tel:+15551234"]]],
	  "entities":[{"roles":["abuse"],"vcardArray":["vcard",[["email",{},"text","abuse@acme.org"],["fn",{},"text","REDACTED FOR PRIVACY"]]]},
	             {"roles":["noc"],"vcardArray":["vcard",[["fn",{},"text","REDACTED"]]]}]}`
	var e rEntity
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	got := flatten(e)
	if len(got) != 2 {
		t.Fatalf("want 2 contacts (the fully redacted one is dropped), got %+v", got)
	}
	c := got[0]
	if c.Name != "Jane Doe" || c.Org != "Acme Inc" || c.Email != "jane@acme.org" || c.Phone != "+15551234" {
		t.Errorf("first contact: %+v", c)
	}
	if got[1].Email != "abuse@acme.org" || got[1].Name != "" {
		t.Errorf("redacted name must be blanked, other fields kept: %+v", got[1])
	}
}
