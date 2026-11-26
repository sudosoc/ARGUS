# Collecting the least that answers the question

ARGUS gathers from public sources only, and by default it gathers **the least that answers the question you chose**, shows you
what that is before anything is sent, and shows what every collector is doing while it runs.

## 1. Choose the question

In the web form, **Question** decides which collectors are proposed. On the command line it is `-goal`.

| Question (`-goal`) | Collectors proposed |
|---|---|
| **Identify who this is** (`identify`) | social, profiles, people, images, activity, devtrace, keys, bskygraph, footprint, phone |
| **Check a claimed identity** (`verify`) | people, profiles, social, records, sanctions, mentions, bizreg, company, devtrace, keys |
| **See what is exposed** (`exposure`) | social, profiles, footprint, phone, breach, images, keys, devtrace, codesearch, mentions, activity |
| **Investigate a domain, IP or company** (`infrastructure`) | domain, ip, asn, sitefiles, docmeta, lookalike, webhistory, company, bizreg, records, syswrap, dorks |
| **Everything available** (`everything`) | every collector that applies to what you entered |

`uploads`, `iddecode`, `crypto` and `context` stay in every question: they only ever work on something you supplied explicitly
(a file, pasted links, a wallet, a place or period). A collector is proposed only if you entered something it can use.

Following leads (a username, e-mail or domain found along the way) stays inside the collectors you ticked. Collectors that copy content
(public posts) are opt-in: they run only when you tick them. See [POSTS.md](POSTS.md).

## 2. See what will be collected

Pressing **Start investigation** does not start anything: it opens **What will be collected**, listing for every proposed collector

* what it is for, in plain words;
* **reads:** which of the details you entered it will use;
* **sends to:** who receives them (or *stays on this machine*);
* **how heavy it is:** few, some or many requests, with a rough total (for example the username sites make one request each).

Untick a collector to drop it, or open *Also available* to add one. Collectors that need something you did not enter are
listed as skipped. Nothing is sent until you press **Start with N collectors**. The question you chose is recorded in the report
(`settings.goal`) so the reader knows what was and was not asked.

On the command line, `argus -plan …` prints the same list and exits without contacting anything:

```bash
argus -u sudosoc -e someone@example.org -goal identify -plan
argus -u sudosoc -e someone@example.org -goal identify
```

Without `-goal` the command line runs every collector that applies (as before); `-modules` still picks collectors by name
and wins over `-goal`.

## 3. Watch what is happening

While an investigation runs, the live panel lists **every collector at the same time** (they all run in parallel, sharing one pool of
workers, so a large collector such as the username sites does not hold the others back). Each row shows:

* its state: *waiting*, *running*, *finished* or *stopped* (with the reason, when a collector fails or crashes: the others carry on);
* **what it last looked at**, for example `checking GitHub`, or `working · 38 of 128 done` for a collector that reports only progress;
* checks done, hits, errors and elapsed time.

On the command line each collector prints `[>] name started` and `[✓] name finished · N found · errors · time`, and the single
progress line lists every collector that is still running with its percentage.

## 4. What is never analysed

Whatever you choose, ARGUS does not draw conclusions about religion or belief, health, sexual orientation or gender identity,
or political and trade-union affiliation. See [INSIGHTS.md](INSIGHTS.md#protected-characteristics).
