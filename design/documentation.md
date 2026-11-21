# Documentation Plan

This document records how TUICast's user documentation is organized, built,
and written. The writing conventions for contributors are in
[`docs/README.md`](../docs/README.md).

## Goals

- A reader who has never automated a terminal can run a working example in
  under 15 minutes.
- An experienced engineer can find a precise fact, such as a flag, key name,
  or error code, in one search or two clicks.
- Search engines and AI answer engines can quote any page section correctly
  without the surrounding page.
- Documentation stays short. Every page answers one question. Readers in our
  audience abandon tools whose documentation is padded.

## Audiences

| Reader | Arrives asking | Starts at |
| --- | --- | --- |
| WMS test engineer | "Can this test our RF/terminal screens?" | README, Concepts |
| Automation developer | "How do I script this in my language?" | Getting started |
| Platform or CI owner | "How do I run this at scale and securely?" | Guides, Security model |
| AI tooling owner | "Can an agent operate our terminal app safely?" | MCP guide, Security model |
| SDK or tool author | "What exactly does the driver accept?" | Reference |

## Structure

The documentation follows the [Diátaxis](https://diataxis.fr/) split between
tutorials (Getting started), how-to guides (Guides), explanation (Concepts),
and reference (Reference). Keeping these separate is what keeps each page
short: a guide links to a concept instead of explaining it, and a concept links
to reference instead of listing every option.

```text
docs/
├── README.md                    writing conventions (not published)
├── hugo.toml                    site configuration (added with the theme)
└── content/
    ├── _index.md                documentation home
    ├── getting-started/
    │   ├── installation.md
    │   ├── reference-tui.md
    │   ├── go.md
    │   ├── python.md
    │   ├── typescript.md
    │   └── java.md
    ├── concepts/
    │   ├── how-tuicast-works.md
    │   ├── connections-and-sessions.md
    │   ├── screens-and-waiting.md
    │   ├── terminal-emulation.md
    │   ├── security-model.md
    │   └── when-to-use-tuicast.md
    ├── guides/
    │   ├── connect-to-your-application.md
    │   ├── diagnose-failed-waits.md
    │   ├── run-sessions-concurrently.md
    │   ├── use-ai-agents-with-mcp.md
    │   ├── record-workflows-with-workbench.md
    │   ├── write-cucumber-tests.md
    │   ├── run-in-containers-and-ci.md
    │   └── embed-the-go-library.md
    ├── reference/
    │   ├── command-line.md
    │   ├── environment-variables.md
    │   ├── matchers.md
    │   ├── keys.md
    │   ├── screen-snapshots.md
    │   ├── driver-protocol.md
    │   ├── mcp-server.md
    │   ├── terminal-support.md
    │   ├── recording-format.md
    │   └── sdk-api.md
    ├── glossary.md
    └── faq.md
```

There is no separate Examples section. Runnable examples already live in
[`examples/`](../examples/README.md), where CI tests them; documentation pages
link to them rather than copying them.

## Page inventory

Status: **Draft** pages have written content to edit and verify. **Brief**
pages contain a writer brief and are marked `draft: true`, so production builds
exclude them until written.

| Page | Answers | Primary sources | Status |
| --- | --- | --- | --- |
| Home | What is TUICast and where do I start? | README | Draft |
| Installation | How do I install the driver, SDKs, and tools? | README | Draft |
| Reference TUI | How do I run the practice application? | README, `design/reference-tui.md` | Draft |
| Go / Python / TypeScript / Java | How do I automate my first login? | existing guides | Draft |
| How TUICast works | What are the driver, SDKs, and MCP server? | `design/architecture.md` | Draft |
| Connections and sessions | How are connections, sessions, and their lifetimes related? | `design/session-lifecycle.md` | Draft |
| Screens and waiting | How do I know when the application is ready? | `design/session-lifecycle.md` | Draft |
| Terminal emulation | What is a terminal profile and which do I choose? | `design/terminal-behavior.md` | Draft |
| Security model | How are credentials, hosts, and agents kept safe? | `design/mcp.md`, `design/driver-ui.md` | Draft |
| When to use TUICast | How does TUICast compare with expect, pexpect, and RPA tools? | external research | Brief |
| Connect to your application | How do I connect to a real SSH or Telnet host? | `design/driver.md` | Draft |
| Diagnose failed waits | Why did my wait time out? | `design/troubleshooting.md` | Draft |
| Run sessions concurrently | How do I run many users at once? | `design/session-lifecycle.md` | Brief |
| Use AI agents with MCP | How do I let an agent operate a terminal app? | `design/mcp.md`, `examples/mcp/` | Brief |
| Record workflows with Workbench | How do I record a test from a live session? | `design/workbench.md` | Brief |
| Write Cucumber tests | How do I write BDD tests for a TUI? | `design/cucumber.md` | Brief |
| Run in containers and CI | How do I run TUICast in Docker and pipelines? | `design/containers.md` | Brief |
| Embed the Go library | How do I use TUICast without the driver process? | root package | Brief |
| Command line | What flags do the binaries accept? | `cmd/*/main.go` | Draft |
| Environment variables | Which variables does TUICast read? | `cmd/`, `design/mcp.md` | Draft |
| Matchers | What can a wait match? | `matcher.go`, `design/driver.md` | Draft |
| Keys | Which named keys and modifiers exist? | `key.go` | Draft |
| Screen snapshots | What does a screen contain? | `screen.go`, `design/driver.md` | Draft |
| Driver protocol | What JSON-RPC methods and errors exist? | `schema/openrpc.json` | Draft |
| MCP server | What are the profile format, secret sources, and tools? | `design/mcp.md` | Draft |
| Terminal support | Which escape sequences are supported? | `design/terminal-behavior.md` | Draft |
| Recording format | What is in a recording file? | `schema/workbench-recording.schema.json` | Brief |
| SDK API | Where is each SDK's API reference? | package registries | Brief |
| Glossary | What does this term mean? | all | Draft |
| FAQ | Short answers to common evaluation questions | all | Draft |

The `design/` documents remain the source of truth for decisions and internal
contracts. User pages restate behavior in user terms and must be updated in the
same change when that behavior changes.

## Page order

Hugo sorts pages by `weight`, then date, then link title, then file path. The
documentation never uses dates for ordering. Every page and every section's
`_index.md` sets `weight` in front matter; lower weights appear first. Weights
use steps of 10 so a page can be inserted without renumbering its neighbors.

Sections are ordered by the weight in their `_index.md`:

| Section | Weight |
| --- | --- |
| Getting started | 10 |
| Concepts | 20 |
| Guides | 30 |
| Reference | 40 |
| Glossary | 50 |
| FAQ | 60 |

The theme builds the sidebar from this content tree, so no separate navigation
file needs to be kept in sync.

## Tooling

**Generator: Hugo.** It is a single binary, which suits a Go project and a CI
pipeline, and needs no Node.js toolchain.

**Theme: Hextra**, imported as a Hugo module pinned in `docs/go.mod`. It
provides a documentation sidebar ordered by `weight`, built-in search, and
`llms` and Markdown output formats, without requiring Node.js. Docsy was the
alternative; it requires Node.js for its PostCSS build. Update the theme with
`hugo mod get -u github.com/imfing/hextra` from `docs/`.

`docs/hugo.toml` applies these choices:

- `enableRobotsTXT` and the built-in sitemap.
- `enableGitInfo`, so each page shows when it was last changed. CI checks out
  full history for this.
- `baseURL` set to `https://tuicast.castingcode.dev/`, so canonical URLs are
  absolute.
- `llms` output on the home page and `markdown` output on pages and sections.
  `docs/layouts/llms.txt` overrides the theme's template to use each page's
  `description`.
- `markup.goldmark.renderHooks.link.useEmbedded = 'fallback'`, so relative
  `.md` links resolve to page URLs. Content uses relative file links so pages
  also read correctly on GitHub.
- Content in `docs/content/` publishes at the site root, for example
  `/concepts/screens-and-waiting/`. The root `_index.md` sets
  `cascade: {type: docs}` to apply Hextra's documentation layout everywhere.

**Hosting: Azure Static Web Apps** (Free plan) at
`https://tuicast.castingcode.dev`. `.github/workflows/docs.yml` builds with a
pinned Hugo version and deploys `docs/public` with
`Azure/static-web-apps-deploy`, authenticating with the
`AZURE_STATIC_WEB_APPS_API_TOKEN` repository secret. Pushes to `main` publish
production. Pull requests from this repository get a preview environment,
removed when the pull request closes; forks are built but not deployed. The
Free plan allows three preview environments at a time.
`docs/static/staticwebapp.config.json` sets the 404 page, Markdown MIME type,
and security headers.

## Discoverability

Search engines (SEO) and AI answer engines (GEO) reward the same things:
specific pages, accurate answers near the top, and consistent terms.

- **One canonical home per fact.** The site is canonical. The README links to
  it instead of duplicating installation and API details.
- **Answer first.** Each page opens with one or two sentences that answer its
  question and name TUICast, because AI engines quote passages out of context.
- **Self-contained sections.** A section should make sense when read alone.
  Repeat the noun ("the driver") rather than "it" at the start of a section.
- **Searchable vocabulary.** Use the words readers search with, such as VT220,
  Telnet, SSH, RF terminal, WMS, terminal emulator, and end-to-end test, in
  titles, headings, and opening sentences when they are accurate.
- **Precise titles and descriptions.** `title` is at most 60 characters.
  `description` is one sentence of 120 to 160 characters and becomes the meta
  description and the `llms.txt` entry.
- **Text, not pictures of text.** Diagrams are text or have a text equivalent.
  Code is in code blocks, never screenshots.
- **Stable URLs.** Never rename a published page without a redirect in the
  Static Web Apps configuration.
- **Structured data.** Add `SoftwareApplication` JSON-LD on the home page.
  `FAQPage` markup is harmless but no longer earns rich results for most sites.
- **`llms.txt`** is a curated map generated from page descriptions. It
  supplements `sitemap.xml`, `robots.txt`, and good HTML; it does not replace
  them.

## Delivery order

1. Edit and verify the drafted Getting started and Concepts pages; they decide
   whether readers adopt TUICast.
2. Edit the drafted Reference pages and the glossary.
3. Write the briefed pages, starting with When to use TUICast and Use AI agents
   with MCP.
4. Once the site is live, point README links at it.
