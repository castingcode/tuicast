# Writing TUICast documentation

This folder holds the source for the TUICast documentation site. Pages live in
`content/` and are written in Markdown. The structure, page inventory, tooling,
and discoverability rules are in
[`design/documentation.md`](../design/documentation.md).

## Where a page belongs

| If the page... | Put it in |
| --- | --- |
| walks a newcomer through a first success | `getting-started/` |
| explains how or why something works | `concepts/` |
| helps a reader complete one real task | `guides/` |
| lists exact facts: flags, fields, codes, names | `reference/` |

If a page needs two of these, split it and link between the pages.

## Front matter

Every page starts with:

```yaml
---
title: "Screens and waiting"
description: "One sentence, 120 to 160 characters, that answers the page's question. Used in search results and llms.txt."
weight: 30
---
```

- `title`: at most 60 characters, in sentence case.
- `description`: written for a search result, not a teaser.
- `weight`: sets the page's position in its section; lower comes first. Use
  steps of 10. A section's position comes from the `weight` in its `_index.md`.
- `linkTitle` (optional): a shorter label for the sidebar.
- `draft: true`: keeps an unfinished page out of production builds. Remove it
  when the page is ready.

Pages that say **Writer brief** describe what to write. Replace the brief with
the page content and remove `draft: true`.

## Writing rules

- **Answer in the first two sentences.** Name TUICast and the subject. A
  reader, search engine, or AI tool should be able to stop there.
- **One page, one question.** If a page grows past about 800 words, it is
  probably two pages.
- **Cut, then cut again.** Remove sentences that announce what follows,
  restate a heading, or praise the product.
- **Use the glossary's terms exactly.** Add a term to
  [`glossary.md`](content/glossary.md) before using it on another page.
- **Link instead of repeating.** Explain a concept once, in `concepts/`, and
  link to it.
- **Make every example runnable.** Prefer linking to tested code in
  `examples/` over writing new snippets. Never put real hostnames or
  passwords in examples; use `example.test` hosts and environment variables.
- **Write links as relative paths to `.md` files**, for example
  `[waits](../concepts/screens-and-waiting.md)`. They work on GitHub and the
  site generator converts them to page URLs.
- **Mark uncertainty.** Leave `<!-- VERIFY: ... -->` comments for facts you
  could not confirm. Engineers will resolve them in review.

## Previewing

Install [Hugo](https://gohugo.io/installation/) (extended edition), then run
from the repository root once `docs/hugo.toml` exists:

```sh
hugo server --source docs --buildDrafts
```

`--buildDrafts` includes pages still marked `draft: true`.
