# Copying public posts

ARGUS can copy what an account has published **in public**, from four places that show it to anyone without an account:
**Mastodon**, **Bluesky**, **GitHub** and **Reddit**. It is the same as opening the profile in a browser where you are not signed in,
done for you and kept with the case. It never signs in, never uses a cookie, a token or a key, and never gets around a limit.

## It is opt-in

These collectors (`postsmastodon`, `postsbluesky`, `postsgithub`, `postsreddit`) never run by default, in any question, including
*Everything available*. In the web UI, **What will be collected** lists them under *Optional: copy public posts*; tick the ones you want.
On the command line name them: `argus -u someone -modules postsgithub,postsbluesky`.

## What it copies, and from whom

* **Only the username and nickname you typed.** Never a guessed or derived name, never an account found along the way.
  `name@server` (for example `ann@infosec.exchange`) names a Mastodon server; without it the largest servers are tried.
* **Mastodon:** up to about 150 public and unlisted posts: text, time, tags, mentions, links, picture addresses, counts.
* **Bluesky:** up to 200 posts and replies from the public feed, shares marked as shares.
* **GitHub:** public activity (commit messages, issues, pull requests, comments, stars) and the public repositories with their topics.
* **Reddit:** public posts and comments, when Reddit shows them to a visitor without an account. Reddit often refuses, and then the
  Sources view says so; ARGUS does not try another way. Adult-marked items are not copied.
* Text is capped at 1,200 characters per item. Pictures are kept as addresses; the picture stage fetches only a few per account
  (for fingerprints and metadata), and the Posts view loads the rest on demand, two at a time.

## What it respects

* An account that **asked not to be shown to people without an account** (Bluesky's logged-out opt-out) is skipped.
* A Mastodon account that **asked not to be indexed** (`noindex`) is skipped.
* Private, followers-only and direct posts are never requested; the first refusal or rate limit **stops that place** and is
  recorded as a coverage gap, not retried around.
* Each place is asked a handful of times, at most one request a second.

Each place's result, including why it is partial or empty, is in **Posts → Sources**.

## What it shows

* **Posts → Posts:** the copied items, searchable, with the original link.
* **Posts → Sources:** what came from each account and why anything is missing.
* **Posts → Activity:** items per month, hour and day of the week, what kinds of items they are.
* **Posts → Topics & people:** most used tags and subjects, who they reply to, share and mention, sites they link to, languages written
  and measurable writing habits (length, links, questions, emoji, capitals).
* The Insights brief mentions how much was copied and what was written about most; languages written feed the persona.

The Markdown report lists the sources and the 40 most recent items; the JSON report has everything (`posts`, `post_sources`).

## What it never analyses

The copied text is evidence and is shown as it was published. But the analysis never lists, and never concludes anything about,
religion or belief, health, sexual orientation or gender identity, or political and trade-union affiliation: tags, subjects, accounts
and sites that touch these are left out (see [INSIGHTS.md](INSIGHTS.md#protected-characteristics)).

## Limits to keep in mind

* Public does not mean harmless. Posts of a private person are personal data; use this only for a lawful, proportionate purpose.
* A feed is a sample (the latest items), not the whole history.
* GitHub allows few requests per hour without a key and other collectors share that allowance; a refusal is reported.
* A copy is a snapshot. Posts the person deleted before the copy are not there; posts deleted after still are.
