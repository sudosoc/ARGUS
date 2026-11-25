# Insights: how the conclusions are made

The **Insights** view turns the evidence into conclusions. Everything in it is computed by fixed rules in `internal/insights`
when an investigation finishes; **no model is involved and nothing is fetched**. The same report always gives the same insights.
They are leads to check, not facts about a person. Each section says what it is based on, and Evidence → *Why this confidence*
shows the reasoning behind every fact.

The result is stored in the report (`insights` in the JSON), appears in the Markdown and HTML reports and in the console summary,
and is saved with the case.

## Brief

A short summary of the best-matching person: who, how well the evidence identifies them (*strong*, *moderate*, *weak*, or *no single person*),
what supports it and what to be careful about. *Strong* needs a match score of 75% or more and no unresolved ambiguity; *moderate* 50% or more.

## Next actions

Rules over the report, ordered by priority (1–100), each with why, how and what it improves:

| Rule | Priority |
|---|---|
| Possible watch-list match found: compare by hand before concluding | 96 |
| Several people match and no location given: add a location | 92 |
| Nothing found: add inputs and check coverage | 90 |
| A picture is used under a different name: check for an impersonator | 88 |
| Several people match: add an employer or school; add an e-mail or username | 84 / 80 |
| Identity looks thin or doubtful: verify it is a real person | 78 |
| Sources disagree on an attribute: resolve it | 72 |
| Exposed or takeover-prone asset found: fix it | 70 |
| Two accounts look linked (strength ≥ 0.7): confirm | 66 |
| Many checks failed (over 15%): re-run later or through a proxy | 62 |
| Manual leads (login, CAPTCHA or payment) for what you supplied | about 58–54 |
| The person's own habits expose a lot: reduce it | 52 |
| An input type is missing that would run more collectors | 48 |

## Persona

* **Languages:** from the text the person wrote about themself (bios, headlines, interests). Scripts (Arabic, Cyrillic, Chinese/Japanese, Korean, Hebrew, Devanagari, Thai, Greek) are exact; among Latin-script languages a small stop-word vote decides (English, French, Spanish, German, Portuguese, Italian, Turkish, Dutch). An undecided text is reported as "Latin script", never guessed. At least 25 letters are needed.
* **Skills and interests:** main languages and topics of public code, stated interests, headlines, ranked by weight.
* **Daily rhythm:** from the activity-hours estimate (needs 20 or more public events): time zone, the quietest six hours, the peak hour, share of activity at night (22:00–05:00 local) and at weekends. Weekdays are counted in UTC.
* **Habits and exposure:** handle reuse across platforms, an e-mail that matches a handle, the real name published widely, the same picture on several accounts, pictures that still carry GPS or a camera serial, personal e-mail in public commits, breaches, infostealer logs. The exposure list is what someone could use to profile the person, and what to fix when checking your own exposure.
* **First and latest trace:** the oldest account creation and the latest dated activity in the timeline.

## Authenticity

A score from 0 to 100, starting at 50, with every factor listed (points added or removed):

| Signal | Points |
|---|---|
| Oldest dated account 8+ years / 3+ years / under 1 year | +15 / +8 / −12 |
| 4+ / 2–3 independent platforms / a single source | +10 / +5 / −10 |
| Name confirmed by two platforms / no profile states a name | +8 / −6 |
| Third-party records (registers, research databases) | +10 |
| Pages written by the person | +5 |
| A published OpenPGP key | +5 |
| The same bio under different names | −15 |
| A picture also used under a different name / a picture of their own | −15 / +3 |
| Contradictions between sources | −4 each, at most −12 |
| Dated activity in 3 or more different years | +6 |

Labels: **Established** 75+, **Plausible** 55–74, **Thin** 35–54, **Doubtful** below 35. It also flags individual accounts (no name, empty profile, a picture reused under another name, created in the last 90 days).
It is a set of observable signals. A new real person scores low; a careful fake can score well.

## Key associates

Ranked by strength: Bluesky mutual follows (0.8) and accounts followed by many of the subject's follows (0.3–0.8); employers and schools stated by profiles (more sources and confirmation raise it); public GitHub organisations. Independent evidence combines. A public connection is a lead about interaction, not proof of a relationship. Associates need a Bluesky account or profiles that state organisations; otherwise the section says so.

## Hidden links

Pairs of accounts that nothing stated as linked but that share: a **handle pattern** (the same letters with different decoration: `lhassan`, `l.hassan`, `lhassan89`; 0.55), **similar text** (at least 45% of overlapping three-word phrases in their bios), a **shared value** (the same e-mail, phone or personal website on different platforms; 0.85), or a **similar picture** (fingerprints differing in at most 10 of 64 bits but not identical: the same photo cropped or recoloured). Strong ones are worth confirming by hand.

## Life timeline

Every dated fact in order, facts per year and the unusual stretches: a **burst** (three or more accounts from different sources created within 30 days), a **gap** (two years or more without a dated trace, or a latest trace older than two years), **recent** activity (within 60 days) and a single dated fact. A gap can mean the person was offline, or that the evidence for those years was not collected.

## Movement

From pictures that carry GPS and a date, and from the location evidence. Sightings within 50 km form a place; the most supported is the home region (the location analysis's best place wins); moves between places are listed in date order with the distance. Fewer than three dated sightings is thin evidence and the section says so. A place is where evidence points, not where the person is now; GPS in a picture can be copied from elsewhere.

## Why this confidence

For each fact about the best-matching person: **confirmed** (two or more independent platforms), **self-reported** (only the person's own profile), **single source** (one third-party record) or **contested** (sources disagree), with the sources, the source reliability and the earliest dated trace.

## Limits

* Insights describe the **best-matching person**. If none could be resolved, only the sections that do not need one are filled.
* They are only as good as the evidence: a sparse run gives sparse insights, and the *Limits* line says which section lacked what.
* The sentences are English in every language setting (the labels and headings follow the interface language).
* Reports and exports always include them; they never leave your machine.
