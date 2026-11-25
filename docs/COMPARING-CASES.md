# Comparing investigations

Every finished investigation is saved automatically as a *case*. Comparing two cases shows what changed between two runs of the same subject: a new account, a domain that dropped a tracker, a person who changed employer, a location that moved.

## Where cases live

A case is a JSON file named `<start time>_<random>.json` in the cases folder, mode 0600:

- default: `~/.config/argus/cases` (your OS's user config directory),
- override with the `ARGUS_CASES` environment variable.

A case is the complete report, so it contains personal data. Keep the folder private and delete cases you no longer need (the *Saved cases* list has a Delete button). Notes you add in the UI are written back into the same file.

## Comparing in the web UI

1. Run the investigation. Later, run it again with the same inputs (the title of a case is its subject, so runs of the same target group together).
2. Open **Saved cases** (top bar) → tick two cases → **Compare the two ticked cases**.
3. The comparison drawer shows:
   - which case is older and which newer (by start time);
   - **Location changed** when the most likely place differs;
   - **Best match changed** when the best-matching person differs;
   - **Entities** (e-mails, domains, accounts…) added and removed, as `+n / −n`;
   - **Findings** added and removed.

A faster way in: open a case and go to **Timeline → Historical changes**. It lists earlier saved runs of the same target with how many findings each had and the difference to the current run, and a button that opens the comparison.

## What counts as "changed"

- An **entity** is identified by its type and value (`email: l.hassan@example.org`). It is "added" if the newer case has it and the older does not.
- A **finding** is identified by its collector, source and URL. Only findings with status *found* or *uncertain* are compared; errors and not-found results are ignored, so a rate-limited check does not look like a disappearance.
- A change *inside* a finding (the same page now says something different) is **not** detected: the finding keeps the same identity. For that, compare the raw JSON of the two cases.

## How to read a difference

A difference is a lead, not a conclusion.

- **Added** may mean the subject created something new, *or* a source that was blocked last time answered this time, *or* the collector set differed (a different `-site-tier`, a module turned off). Check the *Run settings* panel of each case (Evidence → Collection coverage → *Run settings*) and the failures list before reading meaning into it.
- **Removed** may mean something was deleted, *or* a source failed this time. A removal with failures on the same collector is probably a coverage gap, not a deletion.
- Searching is not reproducible: public services change, rate-limit and go offline. Two runs minutes apart can differ.

For a fair comparison, use the same inputs, the same `-site-tier` and the same modules, and note whether proxies were used.

## Comparing from the command line

Cases are plain JSON, so any tool works. For example, with `jq`:

```bash
# entities in the newer case that the older one lacks
jq -r '.entities[] | "\(.type): \(.value)"' older.json | sort -u > a.txt
jq -r '.entities[] | "\(.type): \(.value)"' newer.json | sort -u > b.txt
comm -13 a.txt b.txt
```

```bash
# findings (found or uncertain) by collector, source and URL
jq -r '.findings[] | select(.status=="found" or .status=="uncertain") | "\(.module) / \(.source) \(.url // "")"' case.json | sort -u
```

The report also carries an integrity digest (`integrity.digest`, SHA-256 of target, entities and findings). If two exports of the same case have the same digest, nothing changed in them.

## Comparing people

A different feature compares two *candidate people* inside one investigation (the Identity view, tick two people → **Compare**): shared facts are marked `=`, facts only one has `≠`. It answers "are these the same person?" within one run, not "what changed over time".

## Limits

- Cases are compared in the browser; there is no scheduled re-run or alert when something changes yet.
- Both cases must be on the same machine's cases folder.
- Only entities and findings are compared, not pictures' contents, notes or the timeline.
