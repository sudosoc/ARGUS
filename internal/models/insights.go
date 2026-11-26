// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package models

// Insights is what the analysis stage concludes from the evidence: a short brief, what to do next,
// a profile of the person's online habits, how authentic the identity looks, who is around them,
// links nobody stated, a life timeline with anomalies, movement and the reasoning behind each fact.
// It is derived by fixed, documented rules from the rest of the report (no model is involved),
// so the same report always gives the same insights.
type Insights struct {
	Brief        Brief         `json:"brief"`
	Actions      []Action      `json:"actions,omitempty"`
	Persona      *Persona      `json:"persona,omitempty"`
	Authenticity *Authenticity `json:"authenticity,omitempty"`
	Associates   []Associate   `json:"associates,omitempty"`
	HiddenLinks  []HiddenLink  `json:"hidden_links,omitempty"`
	Life         *LifeTimeline `json:"life,omitempty"`
	Movement     *Movement     `json:"movement,omitempty"`
	Content      *Content      `json:"content,omitempty"`
	Explain      []Explanation `json:"explain,omitempty"`
	Limits       []string      `json:"limits,omitempty"` // what the evidence did not allow
}

// Brief is the one-screen summary.
type Brief struct {
	Headline   string   `json:"headline"`
	Level      string   `json:"level"` // strong | moderate | weak | none: how well the evidence identifies one person
	Paragraphs []string `json:"paragraphs,omitempty"`
	Strengths  []string `json:"strengths,omitempty"`
	Concerns   []string `json:"concerns,omitempty"`
}

// Action is one recommended next step.
type Action struct {
	Priority int    `json:"priority"` // 1-100
	Kind     string `json:"kind"`     // narrow | verify | resolve | collect | follow | defend | rerun
	Title    string `json:"title"`
	Why      string `json:"why"`
	How      string `json:"how"`
	Gain     string `json:"gain"` // what it would improve
	URL      string `json:"url,omitempty"`
}

// Labelled is a value with a weight, used for languages, skills and similar lists.
type Labelled struct {
	Label string  `json:"label"`
	Share float64 `json:"share,omitempty"` // 0-1 where it means a share; otherwise a count in N
	N     int     `json:"n,omitempty"`
	Note  string  `json:"note,omitempty"`
}

// Persona describes how the person appears online.
type Persona struct {
	Languages   []Labelled `json:"languages,omitempty"`
	Expertise   []Labelled `json:"expertise,omitempty"`
	Schedule    *Schedule  `json:"schedule,omitempty"`
	Habits      []string   `json:"habits,omitempty"`   // observed behaviours (handle reuse, photos…)
	Exposure    []string   `json:"exposure,omitempty"` // what the person's own habits give away
	FirstTrace  string     `json:"first_trace,omitempty"`
	FirstSource string     `json:"first_source,omitempty"`
	LatestTrace string     `json:"latest_trace,omitempty"`
	YearsOnline float64    `json:"years_online,omitempty"`
	Accounts    int        `json:"accounts,omitempty"`
	Platforms   []string   `json:"platforms,omitempty"`
}

// Schedule is the daily rhythm read from when the person is active online.
type Schedule struct {
	UTCOffset    float64 `json:"utc_offset"`
	Zone         string  `json:"zone"`
	SleepFrom    int     `json:"sleep_from"` // local hour the quietest window starts
	SleepTo      int     `json:"sleep_to"`
	PeakHour     int     `json:"peak_hour"`
	NightShare   float64 `json:"night_share"`   // share of activity between 22:00 and 05:00 local
	WeekendShare float64 `json:"weekend_share"` // share on Saturday and Sunday (UTC days)
	Events       int     `json:"events"`
	Pattern      string  `json:"pattern"`
}

// Factor is one reason that raised or lowered a score.
type Factor struct {
	Name   string `json:"name"`
	Effect int    `json:"effect"` // points added or removed
	Detail string `json:"detail"`
}

// AccountFlag lists what is odd or reassuring about one account.
type AccountFlag struct {
	Platform string   `json:"platform"`
	URL      string   `json:"url,omitempty"`
	Flags    []string `json:"flags"`
}

// Authenticity estimates whether the identity behaves like an established real person or a thin or fabricated one.
// It is a set of observable signals, not a verdict.
type Authenticity struct {
	Score    int           `json:"score"` // 0-100
	Label    string        `json:"label"` // Established | Plausible | Thin | Doubtful
	Factors  []Factor      `json:"factors"`
	Accounts []AccountFlag `json:"accounts,omitempty"`
}

// Associate is someone or something close to the subject.
type Associate struct {
	Kind     string   `json:"kind"` // person | organisation | school | site
	Name     string   `json:"name"`
	Strength float64  `json:"strength"` // 0-1
	Why      string   `json:"why"`
	Sources  []string `json:"sources,omitempty"`
	URL      string   `json:"url,omitempty"`
}

// HiddenLink connects two accounts or records that nothing stated as linked.
type HiddenLink struct {
	A        string   `json:"a"`
	B        string   `json:"b"`
	Kind     string   `json:"kind"` // handle pattern | similar text | shared value | similar picture
	Strength float64  `json:"strength"`
	Evidence []string `json:"evidence"`
}

// LifeEvent is a dated fact in the chronology.
type LifeEvent struct {
	Date   string `json:"date"`
	Event  string `json:"event"`
	Source string `json:"source,omitempty"`
	URL    string `json:"url,omitempty"`
}

// YearCount is the number of dated facts in a year.
type YearCount struct {
	Year int `json:"year"`
	N    int `json:"n"`
}

// Anomaly is something unusual in the chronology.
type Anomaly struct {
	Kind   string `json:"kind"` // burst | gap | recent | single
	Period string `json:"period"`
	Text   string `json:"text"`
}

// LifeTimeline is the chronology with the unusual stretches marked.
type LifeTimeline struct {
	Events    []LifeEvent `json:"events"`
	PerYear   []YearCount `json:"per_year,omitempty"`
	Anomalies []Anomaly   `json:"anomalies,omitempty"`
	First     string      `json:"first,omitempty"`
	Last      string      `json:"last,omitempty"`
}

// Place is somewhere the subject has been seen.
type Place struct {
	Label   string   `json:"label"`
	Lat     float64  `json:"lat"`
	Lon     float64  `json:"lon"`
	First   string   `json:"first,omitempty"`
	Last    string   `json:"last,omitempty"`
	N       int      `json:"n"`
	Sources []string `json:"sources,omitempty"`
}

// Trip is a move between two places.
type Trip struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Date string  `json:"date"`
	Km   float64 `json:"km"`
}

// Movement is where the subject has been, from dated, geotagged evidence.
type Movement struct {
	Home      string   `json:"home,omitempty"`
	Places    []Place  `json:"places,omitempty"`
	Trips     []Trip   `json:"trips,omitempty"`
	Countries []string `json:"countries,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

// Explanation says why a fact is trusted as much as it is.
type Explanation struct {
	Attribute string   `json:"attribute"`
	Value     string   `json:"value"`
	Level     string   `json:"level"` // confirmed | self-reported | single source | contested
	Why       string   `json:"why"`
	Sources   []string `json:"sources,omitempty"`
	Against   []string `json:"against,omitempty"`
	Earliest  string   `json:"earliest,omitempty"`
}

// Content is what the copied public posts add up to. Topics, accounts and sites that touch religion,
// health, sexual orientation or political affiliation are never listed (see package sensitive).
type Content struct {
	Total        int           `json:"total"`
	Authored     int           `json:"authored"` // items the person wrote (not boosts, stars or repositories)
	Sources      []PostSource  `json:"sources,omitempty"`
	Kinds        []Labelled    `json:"kinds,omitempty"`
	PerMonth     []MonthCount  `json:"per_month,omitempty"`
	Hours        [24]int       `json:"hours"`    // authored items by UTC hour
	Weekdays     [7]int        `json:"weekdays"` // Sunday = 0
	PeakHour     int           `json:"peak_hour"`
	Languages    []Labelled    `json:"languages,omitempty"`
	Topics       []Labelled    `json:"topics,omitempty"`
	Interactions []Labelled    `json:"interactions,omitempty"` // accounts the person talks to or shares
	Domains      []Labelled    `json:"domains,omitempty"`
	Style        *WritingStyle `json:"style,omitempty"`
	First        string        `json:"first,omitempty"`
	Last         string        `json:"last,omitempty"`
	Notes        []string      `json:"notes,omitempty"`
}

// MonthCount is items per month (YYYY-MM).
type MonthCount struct {
	Month string `json:"month"`
	N     int    `json:"n"`
}

// WritingStyle describes how the person writes, from measurable habits only.
type WritingStyle struct {
	AvgWords      float64 `json:"avg_words"`
	LinkShare     float64 `json:"link_share"`
	MediaShare    float64 `json:"media_share"`
	QuestionShare float64 `json:"question_share"`
	EmojiShare    float64 `json:"emoji_share"`
	ShoutShare    float64 `json:"shout_share"` // items mostly in capitals
	ReplyShare    float64 `json:"reply_share"` // of authored items, how many answer someone
}
