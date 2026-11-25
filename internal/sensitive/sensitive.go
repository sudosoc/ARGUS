// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package sensitive keeps the analysis away from protected personal characteristics.
//
// ARGUS never draws conclusions about a person's religion or belief, health, sexual orientation
// or gender identity, or political and trade-union affiliation, even when such words appear in
// the evidence: it records what was found, but it does not derive a persona, interests, skills,
// associates, links or a timeline from them, and it never reports them as insights. There is no
// setting to switch this off.
//
// The check is a deliberate, transparent word list (English and Arabic, with common stems), not
// a model. It errs towards withholding: a harmless text that happens to use one of the words is
// left out of the derived views (the evidence itself is untouched). It cannot catch every way of
// saying these things, so it lowers the risk but is not a guarantee.
package sensitive

import (
	"reflect"
	"strings"
	"unicode"
)

// Categories are the protected areas, in the words used in the interface and the documentation.
var Categories = []string{"religion or belief", "health", "sexual orientation or gender identity", "political or trade-union affiliation"}

type rule struct {
	cat     int
	words   []string // exact words
	stems   []string // word prefixes (at least four letters)
	phrases []string // multi-word phrases
}

var rules = []rule{
	{cat: 0,
		words: []string{"god", "allah", "jesus", "christ", "bible", "quran", "koran", "torah", "church", "mosque", "synagogue", "imam", "rabbi", "pastor", "priest", "hijab", "niqab", "kippah", "halal", "kosher", "ramadan", "easter", "eid", "diwali", "sunni", "shia", "shiite", "copt", "coptic", "wiccan", "pagan", "agnostic", "atheist", "atheism", "humanist",
			"الله", "ربنا", "يسوع", "المسيح", "انجيل", "قران", "تورات", "كنيسه", "مسجد", "كنيس", "قس", "قسيس", "راهب", "حجاب", "نقاب", "رمضان", "سني", "شيعي", "قبطي", "ملحد", "الحاد", "اسلام", "اسلامي", "مسلم", "مسلمه", "مسيحي", "مسيحيه", "يهودي", "سلفي", "صوفي", "ازهري"},
		stems:   []string{"pray", "christian", "muslim", "islam", "jewish", "judais", "hindu", "buddhis", "sikh", "catholic", "protestant", "evangelic", "orthodox", "mormon", "pentecost", "jehovah", "theolog", "sermon", "religio", "spiritual", "worship", "scriptur", "scientolog", "baptis", "methodis", "lutheran", "anglican", "salafi", "sufi", "ahmadi", "bahai", "zoroastr", "taoist", "shinto"},
		phrases: []string{"born again", "bar mitzvah", "praise the lord", "inshallah", "alhamdulillah", "mashallah", "subhanallah", "god bless", "في سبيل الله", "ان شاء الله", "الحمد لله", "ما شاء الله"}},
	{cat: 1,
		words: []string{"hiv", "ptsd", "adhd", "ocd", "cancer", "tumor", "tumour", "diabetic", "diabetes", "depressed", "depression", "anxiety", "bipolar", "schizophrenia", "autistic", "autism", "dyslexia", "dyslexic", "epilepsy", "epileptic", "asthma", "arthritis", "migraine", "alzheimer", "alzheimers", "dementia", "disabled", "disability", "handicapped", "illness", "disease", "chronic", "chemo", "chemotherapy", "pregnant", "pregnancy", "miscarriage", "infertility", "ivf", "anorexia", "bulimia", "addiction", "addict", "rehab", "paralysed", "paralyzed", "wheelchair", "terminally", "diagnosed", "diagnosis",
			"سرطان", "ورم", "سكري", "اكتئاب", "توحد", "صرع", "ايدز", "اعاقه", "معاق", "مريض", "مرض", "امراض", "حامل", "ادمان", "مدمن", "شلل", "كرسي متحرك", "الزهايمر", "ربو", "جلطه", "تشخيص"},
		stems:   []string{"cancer", "diabet", "depress", "schizo", "autis", "epilep", "alzheim", "disabilit", "mentally", "psychiatr", "disorder", "syndrome", "leukem", "leukaem", "lymphom", "carcinom", "sclerosis", "parkinson", "tourette", "transplant", "dialysis", "amputee"},
		phrases: []string{"mental health", "medical condition", "health condition", "health issue", "health issues", "living with", "chronic pain", "eating disorder", "panic attack", "suicidal", "self harm", "self-harm", "sober since", "صحه نفسيه", "حاله صحيه", "امراض مزمنه"}},
	{cat: 2,
		words: []string{"gay", "gays", "lesbian", "lesbians", "bisexual", "pansexual", "asexual", "queer", "lgbt", "lgbtq", "lgbtqia", "transgender", "transsexual", "nonbinary", "non-binary", "genderqueer", "genderfluid", "intersex", "homosexual", "homosexuality", "heterosexual", "closeted", "pronouns",
			"مثلي", "مثليه", "مثليين", "شذوذ"},
		stems:   []string{"lgbt", "homosex", "bisex", "transgend", "transsex", "genderqueer", "genderfluid", "heterosex", "drag queen"},
		phrases: []string{"sexual orientation", "gender identity", "coming out", "came out", "drag queen", "drag king", "he/him", "she/her", "they/them", "ميول جنسيه", "هويه جنسيه", "الهويه الجنسيه", "تحول جنسي"}},
	{cat: 3,
		words: []string{"democrat", "democrats", "republican", "republicans", "conservative", "conservatives", "liberal", "liberals", "leftist", "rightist", "socialist", "communist", "marxist", "leninist", "maoist", "fascist", "nazi", "nationalist", "populist", "anarchist", "libertarian", "maga", "tory", "tories", "gop", "brexit", "activist", "activism", "protest", "protester", "protestor", "campaigner", "unionist", "unionised", "unionized", "politics", "political", "politician", "senator", "parliament",
			"حزب", "احزاب", "حزبي", "سياسه", "سياسي", "سياسيه", "انتخابات", "مرشح", "برلمان", "ليبرالي", "علماني", "يساري", "يميني", "اشتراكي", "شيوعي", "ناصري", "اخوان", "اخواني", "ثوري", "ثوره", "ناشط", "معارض", "معارضه", "نقابه", "نقابي"},
		stems:   []string{"democrat", "republican", "conservativ", "socialis", "communis", "marxis", "fascis", "nationalis", "populis", "anarchis", "libertarian", "activis", "political", "politics", "trade-union", "tradeunion", "labour-", "partisan", "ideolog"},
		phrases: []string{"labour party", "green party", "tory party", "trade union", "trade-union", "labor union", "labour union", "union member", "union rep", "shop steward", "left wing", "left-wing", "right wing", "right-wing", "far left", "far right", "far-right", "far-left", "vote for", "voted for", "campaign for", "حزب الله", "نقابه العمال", "الاخوان المسلمين", "اليمين المتطرف", "اقصى اليسار"}},
}

var arabicFold = strings.NewReplacer("أ", "ا", "إ", "ا", "آ", "ا", "ٱ", "ا", "ى", "ي", "ة", "ه", "ؤ", "و", "ئ", "ي", "ـ", "")

// normalise lowercases, folds Arabic letter variants and strips diacritics, and returns the text as
// space-separated words (letters, digits and the two joiners "-" and "/" inside a word are kept).
func normalise(s string) []string {
	s = arabicFold.Replace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '/' || r == '\'':
			b.WriteRune(r)
		case unicode.Is(unicode.Mn, r): // combining marks, including Arabic diacritics
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Fields(b.String())
}

// stripAl removes the Arabic definite article and the common one-letter prefixes so "المسجد"
// and "بالمسجد" meet "مسجد".
func stripAl(w string) []string {
	out := []string{w}
	for _, p := range []string{"ال", "وال", "بال", "لل", "فال", "كال"} {
		if strings.HasPrefix(w, p) && len([]rune(w)) > len([]rune(p))+1 {
			out = append(out, strings.TrimPrefix(w, p))
		}
	}
	if r := []rune(w); len(r) > 3 && (r[0] == 'و' || r[0] == 'ب' || r[0] == 'ل' || r[0] == 'ف') {
		out = append(out, string(r[1:]))
	}
	return out
}

// Match reports which protected categories the text touches (empty if none).
func Match(text string) []string {
	if len(text) < 2 {
		return nil
	}
	words := normalise(text)
	if len(words) == 0 {
		return nil
	}
	// "liberal arts" is a field of study, not a political position.
	joined := " " + strings.Join(words, " ") + " "
	joined = strings.ReplaceAll(joined, " liberal arts ", " ")
	words = strings.Fields(joined)
	var forms []string
	for _, w := range words {
		forms = append(forms, stripAl(w)...)
	}
	var cats []string
	for _, r := range rules {
		hit := false
		for _, f := range forms {
			if hitWord(r, f) {
				hit = true
				break
			}
		}
		if !hit {
			for _, p := range r.phrases {
				if strings.Contains(joined, " "+strings.Join(normalise(p), " ")+" ") {
					hit = true
					break
				}
			}
		}
		if hit {
			cats = append(cats, Categories[r.cat])
		}
	}
	return cats
}

func hitWord(r rule, w string) bool {
	for _, x := range r.words {
		if w == x || w == x+"s" {
			return true
		}
	}
	for _, p := range r.stems {
		if len(w) >= len(p) && strings.HasPrefix(w, p) {
			return true
		}
	}
	return false
}

// Is reports whether the text touches a protected category.
func Is(text string) bool { return len(Match(text)) > 0 }

// Keep returns the items that do not touch a protected category and how many were left out.
func Keep(items []string) (kept []string, withheld int) {
	for _, s := range items {
		if Is(s) {
			withheld++
			continue
		}
		kept = append(kept, s)
	}
	return kept, withheld
}

// Withheld is the placeholder written where a derived string had to be removed.
const Withheld = "[withheld]"

// Scrub walks v (a pointer to a struct, slice or map of derived output) and replaces every string that
// touches a protected category with Withheld. It returns how many strings it replaced. It is the last
// line of defence behind the targeted filters in each analysis, so a new analysis cannot leak by accident.
func Scrub(v any) int {
	n := 0
	scrub(reflect.ValueOf(v), &n)
	return n
}

func scrub(v reflect.Value, n *int) {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			scrub(v.Elem(), n)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				scrub(v.Field(i), n)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			scrub(v.Index(i), n)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			e := v.MapIndex(k)
			if e.Kind() == reflect.String && Is(e.String()) {
				v.SetMapIndex(k, reflect.ValueOf(Withheld).Convert(e.Type()))
				*n++
			} else if e.Kind() == reflect.Ptr || e.Kind() == reflect.Struct || e.Kind() == reflect.Slice {
				scrub(e, n)
			}
		}
	case reflect.String:
		if v.CanSet() && v.String() != Withheld && Is(v.String()) {
			v.SetString(Withheld)
			*n++
		}
	}
}
