/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"html/template"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type config struct {
	root       string
	outDir     string
	modulePath string
	themeStyle string
}

type markdownDoc struct {
	Title    string
	RelPath  string
	Source   string
	HTMLFile string
}

type apiDoc struct {
	ImportPath string
	Title      string
	ShortPath  string
	Section    string
	BodyHTML   template.HTML
	HTMLFile   string
}

type apiPackage struct {
	ImportPath string
	Dir        string
}

type commandHelpRow struct {
	Name        string
	Description string
}

type commandHelpFlag struct {
	Name        string
	Type        string
	Description string
}

type commandHelpDoc struct {
	Title       string
	Command     string
	Synopsis    string
	Summary     []string
	Usage       []string
	Subcommands []commandHelpRow
	Flags       []commandHelpFlag
	ExitCodes   []commandHelpRow
	Raw         string
}

type apiSchema struct {
	Service  apiSchemaService  `json:"service"`
	Imports  []string          `json:"imports,omitempty"`
	Types    []apiSchemaType   `json:"types,omitempty"`
	Requests []apiSchemaMethod `json:"requests,omitempty"`
	Events   []apiSchemaMethod `json:"events,omitempty"`
}

type apiSchemaService struct {
	Name        string     `json:"name"`
	ID          flexJSONID `json:"id"`
	Package     string     `json:"package,omitempty"`
	Description string     `json:"description,omitempty"`
}

type apiSchemaType struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Fields      []apiSchemaField `json:"fields,omitempty"`
}

type apiSchemaField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type apiSchemaMethod struct {
	Name                string     `json:"name"`
	ID                  flexJSONID `json:"id"`
	RequestType         string     `json:"request_type,omitempty"`
	ResponseType        string     `json:"response_type,omitempty"`
	PayloadType         string     `json:"payload_type,omitempty"`
	OneWay              bool       `json:"one_way,omitempty"`
	Description         string     `json:"description,omitempty"`
	RequestDescription  string     `json:"request_description,omitempty"`
	ResponseDescription string     `json:"response_description,omitempty"`
}

type flexJSONID string

func (v *flexJSONID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*v = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*v = flexJSONID(strings.TrimSpace(s))
		return nil
	}
	*v = flexJSONID(strings.TrimSpace(string(data)))
	return nil
}

type docsIndexMeta struct {
	Order  map[string]int
	Titles map[string]string
}

type navEntry struct {
	Title  string
	Href   string
	Active bool
}

type navGroup struct {
	Title   string
	Entries []navEntry
}

type navSection struct {
	Title  string
	Groups []navGroup
}

type pageData struct {
	Head        template.HTML
	CurrentPath string
	Sidebar     []navSection
	BodyHTML    template.HTML
}

type markdownSection struct {
	Title   string
	Content string
}

var (
	titleTagPattern       = regexp.MustCompile(`(?is)<title>.*?</title>`)
	descriptionMetaTag    = regexp.MustCompile(`(?is)<meta[^>]*name=["']description["'][^>]*>`)
	scriptTagPattern      = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	stylesheetLinkPattern = regexp.MustCompile(`(?is)<link[^>]*href=["'][^"']*styles\.css[^"']*["'][^>]*>`)

	markdownLinkPattern = regexp.MustCompile(`(!?\[[^\]]*\]\()([^)]+)(\))`)
	htmlAttrLinkPattern = regexp.MustCompile(`(?i)\b(src|href)=["']([^"']+)["']`)

	mdHeadingPattern      = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdUnorderedPattern    = regexp.MustCompile(`^[-*+]\s+(.+)$`)
	mdOrderedPattern      = regexp.MustCompile(`^\d+\.\s+(.+)$`)
	mdTableDividerPattern = regexp.MustCompile(`^\s*\|?[\s:-]+\|[\s|:-]*$`)
	mdImagePattern        = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	mdLinkPattern         = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdCodePattern         = regexp.MustCompile("`([^`]+)`")
	mdBoldPattern         = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdItalicPattern       = regexp.MustCompile(`\*([^*]+)\*`)
	htmlOpenTagPattern    = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9:-]*)(\s[^>]*)?>$`)
	htmlCloseTagPattern   = regexp.MustCompile(`^</([A-Za-z][A-Za-z0-9:-]*)\s*>$`)
	htmlSelfTagPattern    = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9:-]*)(\s[^>]*)?/\s*>$`)
	commandFlagLineRegex  = regexp.MustCompile(`^\s*-(\S+)(?:\s+(.+))?$`)
)

var pageTemplate = template.Must(template.New("docs-page").Parse(`<!doctype html>
<html lang="en">
{{ .Head }}
<body>
  <header class="site-header">
    <div class="container nav">
      <a class="brand" href="index.html" aria-label="AvyOS docs home">
        <span>AvyOS Docs</span>
      </a>
      <div class="nav-actions">
        <a class="btn btn-release" href="index.html">Documentation Home</a>
      </div>
    </div>
  </header>

  <main class="doc-shell">
    <div class="container doc-layout">
      <aside class="doc-sidebar card" aria-label="Documentation navigation">
        {{ range .Sidebar }}
        <section class="doc-nav-group">
          <h2>{{ .Title }}</h2>
          {{ range .Groups }}
          {{ if .Title }}<p class="doc-subtitle">{{ .Title }}</p>{{ end }}
          {{ range .Entries }}
          <a class="doc-link{{ if .Active }} active{{ end }}" href="{{ .Href }}">{{ .Title }}</a>
          {{ end }}
          {{ end }}
        </section>
        {{ end }}
      </aside>

      <article class="doc-content card">
        {{ .BodyHTML }}
      </article>
    </div>
  </main>
</body>
</html>
`))

const fallbackHead = `<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta name="color-scheme" content="light dark" />
  <title>AvyOS Documentation</title>
  <link rel="stylesheet" href="styles.css" />
</head>`

const docgenStyles = `
.doc-shell .container {
  width: calc(100% - 44px);
  max-width: none;
  margin-inline: auto;
}

.doc-shell {
  padding-block: clamp(2rem, 4vw, 3.2rem);
}

.doc-layout {
  display: grid;
  grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
  gap: var(--space-4);
  align-items: start;
}

.doc-sidebar {
  position: sticky;
  top: 92px;
  max-height: calc(100vh - 112px);
  overflow: auto;
  padding: var(--space-3);
  scrollbar-gutter: stable;
  scrollbar-width: thin;
  scrollbar-color: rgba(13, 99, 243, 0.42) transparent;
}

.doc-sidebar::-webkit-scrollbar {
  width: 12px;
}

.doc-sidebar::-webkit-scrollbar-track {
  background: transparent;
  margin: 3px 0;
  border-radius: 999px;
}

.doc-sidebar::-webkit-scrollbar-thumb {
  border-radius: 999px;
  border: 3px solid transparent;
  background-clip: padding-box;
  background: linear-gradient(180deg, rgba(13, 99, 243, 0.52), rgba(17, 182, 232, 0.46));
  min-height: 34px;
}

.doc-sidebar:hover::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, rgba(13, 99, 243, 0.68), rgba(17, 182, 232, 0.62));
}

.doc-nav-group + .doc-nav-group {
  margin-top: var(--space-3);
}

.doc-nav-group h2 {
  margin: 0 0 0.5rem;
  padding: 0 0.25rem;
  font-size: 0.82rem;
  font-family: var(--display);
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: var(--muted);
}

.doc-subtitle {
  margin: 0.4rem 0 0.2rem;
  padding: 0 0.25rem;
  font-family: var(--mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  color: var(--muted);
}

.doc-link {
  display: block;
  text-decoration: none;
  color: var(--muted);
  padding: 0.5rem 0.66rem;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  transition: background-color 0.14s ease, border-color 0.14s ease, color 0.14s ease;
  font-size: 0.94rem;
  line-height: 1.35;
}

.doc-link:hover {
  color: var(--text);
  border-color: var(--border);
  background: rgba(13, 99, 243, 0.1);
}

.doc-link.active {
  color: var(--text);
  font-weight: 700;
  border-color: rgba(13, 99, 243, 0.3);
  background: rgba(13, 99, 243, 0.16);
}

.doc-content {
  padding: 0;
  min-height: 80vh;
  background: transparent;
  border: 0;
  box-shadow: none;
}

.doc-content.card {
  background: transparent;
  border: 0;
  box-shadow: none;
}

.doc-content > *:first-child {
  margin-top: 0;
}

.doc-path {
  margin-top: 0;
  margin-bottom: 0.7rem;
  letter-spacing: 0.01em;
  font-size: 0.9rem;
}

.doc-list {
  margin: 0.35rem 0 0;
  padding-left: 1.2rem;
}

.doc-list li + li {
  margin-top: 0.4rem;
}

.doc-list a {
  color: var(--accent);
}

.doc-markdown,
.doc-api {
  max-width: none;
}

.doc-markdown h1,
.doc-markdown h2,
.doc-markdown h3,
.doc-markdown h4,
.doc-api h1,
.doc-api h2,
.doc-api h3,
.doc-api h4 {
  font-family: var(--display);
  letter-spacing: -0.02em;
  line-height: 1.14;
  margin-top: 1.35rem;
  margin-bottom: 0.62rem;
}

.doc-markdown h1,
.doc-api h1 {
  margin-top: 0;
  font-size: clamp(1.7rem, 2.5vw, 2.35rem);
}

.doc-markdown h2,
.doc-api h2 {
  font-size: clamp(1.25rem, 1.9vw, 1.68rem);
  padding-top: 0.3rem;
  border-top: 1px solid var(--border);
}

.doc-markdown p,
.doc-api p {
  margin: 0.72rem 0;
  color: var(--text);
  line-height: 1.62;
}

.doc-markdown ul,
.doc-markdown ol,
.doc-api ul,
.doc-api ol {
  margin: 0.72rem 0;
  padding-left: 1.35rem;
}

.doc-markdown li + li,
.doc-api li + li {
  margin-top: 0.3rem;
}

.doc-markdown blockquote,
.doc-api blockquote {
  margin: 0.95rem 0;
  padding: 0.75rem 0.95rem;
  border-left: 4px solid rgba(13, 99, 243, 0.42);
  background: rgba(13, 99, 243, 0.09);
  border-radius: var(--radius-sm);
}

.doc-markdown pre,
.doc-api pre {
  overflow: auto;
  margin: 0.95rem 0;
  padding: 0.95rem 1.05rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: rgba(16, 26, 43, 0.08);
  box-shadow: inset 0 0 0 1px rgba(13, 99, 243, 0.09);
}

.doc-markdown pre code,
.doc-api pre code {
  font-size: 0.9rem;
  line-height: 1.5;
}

@media (prefers-color-scheme: dark) {
  .doc-sidebar {
    scrollbar-color: rgba(17, 182, 232, 0.52) transparent;
  }

  .doc-sidebar::-webkit-scrollbar-track {
    background: transparent;
  }

  .doc-sidebar::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(17, 182, 232, 0.56), rgba(232, 239, 255, 0.34));
  }

  .doc-sidebar:hover::-webkit-scrollbar-thumb {
    background: linear-gradient(180deg, rgba(17, 182, 232, 0.72), rgba(232, 239, 255, 0.44));
  }

  .doc-markdown blockquote,
  .doc-api blockquote {
    border-left-color: rgba(17, 182, 232, 0.45);
    background: rgba(17, 182, 232, 0.11);
  }

  .doc-markdown pre,
  .doc-api pre {
    background: rgba(232, 239, 255, 0.1);
    box-shadow: inset 0 0 0 1px rgba(17, 182, 232, 0.2);
  }

  .doc-ref-hero {
    background: rgba(17, 182, 232, 0.12);
  }

  .doc-ref-card,
  .doc-section-card,
  .api-io-card {
    background: rgba(232, 239, 255, 0.06);
  }

  .api-item-card {
    background: rgba(17, 182, 232, 0.09);
  }

  .doc-ref-table th {
    background: rgba(17, 182, 232, 0.14);
  }

  .doc-ref-code {
    background: rgba(232, 239, 255, 0.1);
  }

  .doc-ref-callout {
    border-left-color: rgba(17, 182, 232, 0.45);
    background: rgba(17, 182, 232, 0.11);
  }
}

.doc-markdown code,
.doc-api code {
  font-family: var(--mono);
  font-size: 0.9rem;
}

.doc-markdown p code,
.doc-markdown li code,
.doc-api p code,
.doc-api li code {
  padding: 0.1rem 0.3rem;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: rgba(13, 99, 243, 0.09);
}

.doc-markdown table {
  width: 100%;
  margin: 0.9rem 0 1.3rem;
  border-collapse: separate;
  border-spacing: 0;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.doc-markdown th,
.doc-markdown td {
  border-bottom: 1px solid var(--border);
  border-right: 1px solid var(--border);
  text-align: left;
  padding: 0.56rem 0.66rem;
  vertical-align: top;
}

.doc-markdown th:last-child,
.doc-markdown td:last-child {
  border-right: 0;
}

.doc-markdown tr:last-child td {
  border-bottom: 0;
}

.doc-markdown th {
  background: rgba(13, 99, 243, 0.08);
  font-family: var(--display);
  font-size: 0.93rem;
}

.doc-markdown a {
  color: var(--accent);
  text-decoration-thickness: 1px;
  text-underline-offset: 2px;
}

.doc-markdown hr,
.doc-api hr {
  border: 0;
  border-top: 1px solid var(--border);
  margin: 1.2rem 0;
}

.doc-markdown img,
.doc-api img {
  max-width: 100%;
  height: auto;
  display: block;
  border-radius: 12px;
  border: 1px solid var(--border);
  box-shadow: var(--shadow-2);
  margin: 0.9rem 0;
}

.doc-ref-shell {
  display: grid;
  gap: 1rem;
}

.doc-ref-hero {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1rem 1.1rem;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 8px 22px rgba(16, 24, 40, 0.08);
}

.doc-ref-hero h1 {
  margin: 0;
  font-family: var(--display);
  font-size: clamp(1.5rem, 2.2vw, 2rem);
  line-height: 1.1;
}

.doc-ref-hero .doc-path {
  margin-top: 0.3rem;
  margin-bottom: 0;
}

.doc-ref-lead {
  margin: 0.55rem 0 0;
  color: var(--text);
  line-height: 1.45;
}

.doc-ref-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 0.95rem;
}

.doc-ref-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  padding: 0.85rem 0.95rem;
  box-shadow: 0 8px 22px rgba(16, 24, 40, 0.08);
}

.doc-ref-card h2 {
  margin: 0 0 0.62rem;
  font-size: 1.05rem;
  font-family: var(--display);
}

.doc-ref-meta {
  margin: 0;
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 0.45rem 0.7rem;
}

.doc-ref-meta dt {
  margin: 0;
  font-size: 0.83rem;
  font-family: var(--mono);
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.doc-ref-meta dd {
  margin: 0;
  color: var(--text);
}

.doc-ref-preview {
  margin: 0;
}

.doc-ref-preview img {
  width: 100%;
  max-width: 560px;
  height: auto;
  border-radius: 10px;
  border: 1px solid var(--border);
  box-shadow: var(--shadow-2);
}

.doc-ref-preview figcaption {
  margin-top: 0.45rem;
  color: var(--muted);
  font-size: 0.86rem;
}

.doc-ref-empty {
  margin: 0;
  color: var(--muted);
  font-style: italic;
}

.doc-ref-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
}

.doc-ref-table th,
.doc-ref-table td {
  text-align: left;
  border-bottom: 1px solid var(--border);
  border-right: 1px solid var(--border);
  padding: 0.5rem 0.62rem;
  vertical-align: top;
}

.doc-ref-table th:last-child,
.doc-ref-table td:last-child {
  border-right: 0;
}

.doc-ref-table tr:last-child td {
  border-bottom: 0;
}

.doc-ref-table th {
  font-family: var(--display);
  font-size: 0.9rem;
  background: rgba(13, 99, 243, 0.1);
}

.doc-ref-list {
  margin: 0;
  padding-left: 1.2rem;
}

.doc-ref-list li + li {
  margin-top: 0.35rem;
}

.doc-ref-code {
  margin: 0;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 0.7rem 0.8rem;
  background: rgba(16, 26, 43, 0.08);
}

.doc-ref-callout {
  margin: 0;
  border: 1px solid var(--border);
  border-left: 4px solid rgba(13, 99, 243, 0.44);
  border-radius: 10px;
  padding: 0.68rem 0.78rem;
  background: rgba(13, 99, 243, 0.08);
  color: var(--text);
}

.doc-card-stack {
  display: grid;
  gap: 1rem;
}

.doc-section-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  padding: 0.9rem 1rem;
  box-shadow: 0 8px 22px rgba(16, 24, 40, 0.08);
}

.doc-section-card > h2 {
  margin: 0 0 0.55rem;
  font-family: var(--display);
  font-size: 1.15rem;
}

.api-card-list {
  display: grid;
  gap: 0.9rem;
}

.api-item-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.92);
  padding: 0.75rem 0.8rem;
  box-shadow: 0 6px 18px rgba(16, 24, 40, 0.07);
}

.api-item-card > h3,
.api-item-head h3 {
  margin: 0;
  font-family: var(--display);
  font-size: 1.02rem;
}

.api-item-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.6rem;
}

.api-item-id {
  margin: 0;
}

.api-io-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.75rem;
  margin-top: 0.75rem;
}

.api-io-card {
  border: 1px solid var(--border);
  border-radius: 9px;
  background: rgba(255, 255, 255, 0.96);
  padding: 0.65rem 0.72rem;
}

.api-io-card h4 {
  margin: 0 0 0.35rem;
  font-family: var(--display);
  font-size: 0.98rem;
}

.api-io-card p {
  margin: 0.4rem 0;
}

@media (max-width: 1080px) {
  .doc-shell .container {
    width: calc(100% - 30px);
  }

  .doc-sidebar {
    top: 82px;
    max-height: calc(100vh - 100px);
  }
}

@media (max-width: 940px) {
  .doc-layout {
    grid-template-columns: 1fr;
  }

  .doc-sidebar {
    position: static;
    max-height: none;
    overflow: visible;
  }
}
`

func main() {
	var cfg config
	flag.StringVar(&cfg.outDir, "out", "_cache/docs", "Output directory for generated docs")
	flag.StringVar(&cfg.modulePath, "module", "", "Go module path (auto-detected from go.mod if empty)")
	flag.StringVar(&cfg.themeStyle, "theme-style", "docs/styles.css", "Theme source styles.css")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
	cfg.root = root

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.modulePath == "" {
		path, err := detectModulePath(filepath.Join(cfg.root, "go.mod"))
		if err != nil {
			return err
		}
		cfg.modulePath = path
	}

	outDir := resolvePath(cfg.root, cfg.outDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(outDir, "project-index.html"))
	_ = os.Remove(filepath.Join(outDir, "project-readme.html"))

	projectDocs, err := collectProjectDocs(cfg.root)
	if err != nil {
		return err
	}
	indexMeta := parseDocsIndex(projectDocs)

	homeRelPath := ""
	if _, ok := findProjectDoc(projectDocs, "docs/index.md"); ok {
		homeRelPath = "docs/index.md"
	} else if _, ok := findProjectDoc(projectDocs, "README.md"); ok {
		homeRelPath = "README.md"
	}

	markdownMap := map[string]string{}
	for i := range projectDocs {
		if projectDocs[i].RelPath == homeRelPath {
			markdownMap[projectDocs[i].RelPath] = "index.html"
		} else {
			markdownMap[projectDocs[i].RelPath] = projectDocs[i].HTMLFile
		}
	}

	appDocs, err := renderAppDocs(cfg.root)
	if err != nil {
		return err
	}
	commandDocs, err := renderCommandDocs(cfg.root)
	if err != nil {
		return err
	}
	serviceDocs, err := renderProgramDocs(cfg.root, "services", []string{"docs.go", "doc.go"})
	if err != nil {
		return err
	}
	apiDocs, err := renderAPIJSONDocs(cfg.root)
	if err != nil {
		return err
	}
	pkgDocs, err := renderPkgDocs(cfg.root, cfg.modulePath)
	if err != nil {
		return err
	}
	allGeneratedDocs := make([]apiDoc, 0, len(appDocs)+len(commandDocs)+len(serviceDocs)+len(apiDocs)+len(pkgDocs))
	allGeneratedDocs = append(allGeneratedDocs, appDocs...)
	allGeneratedDocs = append(allGeneratedDocs, commandDocs...)
	allGeneratedDocs = append(allGeneratedDocs, serviceDocs...)
	allGeneratedDocs = append(allGeneratedDocs, apiDocs...)
	allGeneratedDocs = append(allGeneratedDocs, pkgDocs...)

	if err := copyFile(resolvePath(cfg.root, cfg.themeStyle), filepath.Join(outDir, "styles.css")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "docgen.css"), []byte(docgenStyles), 0644); err != nil {
		return err
	}
	if err := copyDirIfExists(filepath.Join(cfg.root, "docs", "assets"), filepath.Join(outDir, "assets")); err != nil {
		return err
	}
	if err := copyAppPreviewAssets(cfg.root, outDir); err != nil {
		return err
	}
	if err := copyOptionalFile(filepath.Join(cfg.root, "data", "icons", "logo", "logo.png"), filepath.Join(outDir, "assets", "logo.png")); err != nil {
		return err
	}

	for _, d := range projectDocs {
		if d.RelPath == homeRelPath {
			continue
		}
		rewritten := rewriteDocLinks(d.Source, d.RelPath, markdownMap)
		body := renderProjectBody(d.Title, d.RelPath, rewritten)
		head := buildHead(d.Title+" | AvyOS Docs", "Project documentation for "+d.Title)
		page := pageData{
			Head:        head,
			CurrentPath: d.HTMLFile,
			Sidebar:     buildSidebar(projectDocs, allGeneratedDocs, indexMeta, homeRelPath, d.HTMLFile),
			BodyHTML:    body,
		}
		if err := renderPage(filepath.Join(outDir, d.HTMLFile), page); err != nil {
			return err
		}
	}

	for _, d := range allGeneratedDocs {
		body := renderAPIBody(d)
		head := buildHead(d.ImportPath+" | AvyOS Docs", "Documentation for "+d.ImportPath)
		page := pageData{
			Head:        head,
			CurrentPath: d.HTMLFile,
			Sidebar:     buildSidebar(projectDocs, allGeneratedDocs, indexMeta, homeRelPath, d.HTMLFile),
			BodyHTML:    body,
		}
		if err := renderPage(filepath.Join(outDir, d.HTMLFile), page); err != nil {
			return err
		}
	}

	var homeBody template.HTML
	homeTitle := "Project Documentation | AvyOS Docs"
	homeDesc := "Project documentation and API reference for AvyOS"
	if homeRelPath != "" {
		homeDoc, _ := findProjectDoc(projectDocs, homeRelPath)
		rewritten := rewriteDocLinks(homeDoc.Source, homeDoc.RelPath, markdownMap)
		homeBody = renderReadmeIndexBody(homeDoc.RelPath, rewritten)
		homeTitle = homeDoc.Title + " | AvyOS Docs"
		homeDesc = "Documentation index for AvyOS"
	} else {
		homeBody = renderProjectIndexBody(projectDocs)
	}
	home := pageData{
		Head:        buildHead(homeTitle, homeDesc),
		CurrentPath: "index.html",
		Sidebar:     buildSidebar(projectDocs, allGeneratedDocs, indexMeta, homeRelPath, "index.html"),
		BodyHTML:    homeBody,
	}
	if err := renderPage(filepath.Join(outDir, "index.html"), home); err != nil {
		return err
	}

	fmt.Printf("[*] Project docs: %d\n", len(projectDocs))
	fmt.Printf("[*] Generated docs: %d\n", len(allGeneratedDocs))
	fmt.Printf("[✓] Documentation site generated at %s\n", filepath.Join(outDir, "index.html"))
	return nil
}

func collectProjectDocs(root string) ([]markdownDoc, error) {
	var files []string

	readme := filepath.Join(root, "README.md")
	if st, err := os.Stat(readme); err == nil && !st.IsDir() {
		files = append(files, readme)
	}

	docsDir := filepath.Join(root, "docs")
	if st, err := os.Stat(docsDir); err == nil && st.IsDir() {
		if err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(d.Name()), ".md") {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	if len(files) == 0 {
		return nil, errors.New("no markdown files found (expected README.md and/or docs/*.md)")
	}

	usedNames := map[string]int{}
	docs := make([]markdownDoc, 0, len(files))
	for _, p := range files {
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		title := markdownTitle(string(content), strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)))
		baseName := "project-" + slugify(strings.TrimSuffix(rel, filepath.Ext(rel)))
		htmlName := uniqueHTMLName(baseName, usedNames)
		docs = append(docs, markdownDoc{
			Title:    title,
			RelPath:  rel,
			Source:   string(content),
			HTMLFile: htmlName,
		})
	}

	return docs, nil
}

func collectGoPackages(root, modulePath, prefix string, includeMain bool) ([]apiPackage, error) {
	var packages []apiPackage
	searchRoot := filepath.Join(root, prefix)
	st, err := os.Stat(searchRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		pkgName, hasGoFiles, err := packageNameForDir(path)
		if err != nil {
			return err
		}
		if !hasGoFiles {
			return nil
		}
		if includeMain {
			if pkgName != "main" {
				return nil
			}
		} else if pkgName == "main" {
			return nil
		}

		importPath := modulePath
		if rel != "." {
			importPath += "/" + rel
		}
		packages = append(packages, apiPackage{
			ImportPath: importPath,
			Dir:        path,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].ImportPath < packages[j].ImportPath
	})
	return packages, nil
}

func collectMainProgramDirs(root, prefix string) ([]string, error) {
	searchRoot := filepath.Join(root, prefix)
	st, err := os.Stat(searchRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var dirs []string
	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		mainFile := filepath.Join(path, "main.go")
		if _, statErr := os.Stat(mainFile); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return nil
			}
			return statErr
		}

		pkgName, hasGoFiles, pkgErr := packageNameForDir(path)
		if pkgErr != nil {
			return pkgErr
		}
		if !hasGoFiles || pkgName != "main" {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			return nil
		}
		seen[rel] = struct{}{}
		dirs = append(dirs, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(dirs)
	return dirs, nil
}

func renderProgramDocs(root, section string, preferredDocFiles []string) ([]apiDoc, error) {
	dirs, err := collectMainProgramDirs(root, section)
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(dirs))
	for _, relDir := range dirs {
		title := filepath.Base(relDir)
		docText, sourceFile, readErr := readProgramDoc(root, relDir, preferredDocFiles)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read docs for %s: %v\n", relDir, readErr)
		}

		baseName := "api-" + slugify(relDir)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: relDir,
			Title:      title,
			ShortPath:  relDir,
			Section:    section,
			BodyHTML:   renderProgramReferenceBody(section, title, relDir, docText, sourceFile),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func renderAppDocs(root string) ([]apiDoc, error) {
	dirs, err := collectMainProgramDirs(root, "apps")
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(dirs))
	for _, relDir := range dirs {
		title := filepath.Base(relDir)
		docText, sourceFile, readErr := readProgramDoc(root, relDir, []string{"docs.go", "doc.go"})
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read docs for %s: %v\n", relDir, readErr)
		}

		previewPath := appPreviewPath(root, relDir)
		usage := ""
		captureErr := ""
		usesFlags, detectErr := commandUsesFlagPackage(filepath.Join(root, relDir))
		if detectErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to inspect app flags for %s: %v\n", relDir, detectErr)
			usesFlags = true
		}
		if usesFlags {
			captured, usageErr := commandUsageText(root, relDir)
			if usageErr != nil {
				fmt.Fprintf(os.Stderr, "docgen: warning: failed to capture app help for %s: %v\n", relDir, usageErr)
				captureErr = usageErr.Error()
			}
			usage = captured
		}
		help := parseCommandHelp(strings.TrimSpace(usage))

		baseName := "api-" + slugify(relDir)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: relDir,
			Title:      title,
			ShortPath:  relDir,
			Section:    "apps",
			BodyHTML:   renderAppReferenceBody(title, relDir, docText, sourceFile, previewPath, help, usesFlags, captureErr),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func appPreviewPath(root, relDir string) string {
	name := strings.TrimSpace(filepath.Base(relDir))
	if name == "" {
		return ""
	}
	sourcePath := filepath.Join(root, relDir, "preview.png")
	if _, err := os.Stat(sourcePath); err != nil {
		return ""
	}
	return "assets/apps/" + name + "/preview.png"
}

func renderCommandDocs(root string) ([]apiDoc, error) {
	dirs, err := collectMainProgramDirs(root, "cmd")
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(dirs))
	for _, relDir := range dirs {
		title := filepath.Base(relDir)
		docText, sourceFile, readErr := readProgramDoc(root, relDir, []string{"doc.go", "docs.go"})
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to read docs for %s: %v\n", relDir, readErr)
		}

		usage := ""
		captureErr := ""
		usesFlags, detectErr := commandUsesFlagPackage(filepath.Join(root, relDir))
		if detectErr != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: failed to inspect command flags for %s: %v\n", relDir, detectErr)
			usesFlags = true
		}
		if usesFlags {
			captured, usageErr := commandUsageText(root, relDir)
			if usageErr != nil {
				fmt.Fprintf(os.Stderr, "docgen: warning: failed to capture flag.Usage for %s: %v\n", relDir, usageErr)
				captureErr = usageErr.Error()
			}
			usage = captured
		}
		usage = strings.TrimSpace(usage)
		parsed := parseCommandHelp(usage)

		baseName := "api-" + slugify(relDir)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: relDir,
			Title:      title,
			ShortPath:  relDir,
			Section:    "cmd",
			BodyHTML:   renderCommandReferenceBody(title, relDir, docText, sourceFile, parsed, usesFlags, captureErr),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func commandUsesFlagPackage(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			return false, readErr
		}
		source := string(content)
		if strings.Contains(source, `"flag"`) || strings.Contains(source, "flag.") {
			return true, nil
		}
	}
	return false, nil
}

func renderAPIJSONDocs(root string) ([]apiDoc, error) {
	apiRoot := filepath.Join(root, "api")
	st, err := os.Stat(apiRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, nil
	}

	var files []string
	if err := filepath.WalkDir(apiRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "api.json") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)

	used := map[string]int{}
	out := make([]apiDoc, 0, len(files))
	for _, path := range files {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}

		var spec apiSchema
		if unmarshalErr := json.Unmarshal(content, &spec); unmarshalErr != nil {
			return nil, fmt.Errorf("%s: %w", path, unmarshalErr)
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil, relErr
		}
		rel = filepath.ToSlash(rel)
		shortPath := strings.TrimSuffix(rel, "/api.json")
		shortPath = strings.TrimSuffix(shortPath, ".json")
		shortPath = filepath.ToSlash(strings.TrimSpace(shortPath))
		if shortPath == "" || shortPath == "." {
			continue
		}

		title := filepath.Base(shortPath)
		importPath := shortPath
		if name := strings.TrimSpace(spec.Service.Name); name != "" {
			importPath = name
		}
		if pkg := strings.TrimSpace(spec.Service.Package); pkg != "" {
			title = pkg
		}

		baseName := "api-" + slugify(shortPath)
		htmlName := uniqueHTMLName(baseName, used)
		out = append(out, apiDoc{
			ImportPath: importPath,
			Title:      title,
			ShortPath:  shortPath,
			Section:    "api",
			BodyHTML:   renderAPIJSONBody(rel, spec),
			HTMLFile:   htmlName,
		})
	}

	return out, nil
}

func renderPkgDocs(root, modulePath string) ([]apiDoc, error) {
	packages, err := collectGoPackages(root, modulePath, "pkg", false)
	if err != nil {
		return nil, err
	}

	used := map[string]int{}
	out := make([]apiDoc, 0, len(packages))

	for _, pkg := range packages {
		bodyHTML, err := renderPackageBody(pkg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docgen: warning: skipping %s: %v\n", pkg.ImportPath, err)
			continue
		}

		baseName := "api-" + slugify(pkg.ImportPath)
		htmlName := uniqueHTMLName(baseName, used)
		shortPath := shortenImportPath(modulePath, pkg.ImportPath)
		out = append(out, apiDoc{
			ImportPath: pkg.ImportPath,
			Title:      shortPath,
			ShortPath:  shortPath,
			Section:    "pkg",
			BodyHTML:   bodyHTML,
			HTMLFile:   htmlName,
		})
	}
	return out, nil
}

func readProgramDoc(root, relDir string, preferredDocFiles []string) (docText, sourceFile string, err error) {
	for _, name := range preferredDocFiles {
		candidate := filepath.Join(root, relDir, name)
		st, statErr := os.Stat(candidate)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return "", "", statErr
		}
		if st.IsDir() {
			continue
		}

		docText, parseErr := packageDocFromFile(candidate)
		if parseErr != nil {
			return "", "", parseErr
		}
		docText = strings.TrimSpace(docText)
		if docText == "" {
			continue
		}
		rel, relErr := filepath.Rel(root, candidate)
		if relErr != nil {
			return docText, "", nil
		}
		return docText, filepath.ToSlash(rel), nil
	}

	docText, parseErr := packageDocForDir(filepath.Join(root, relDir), relDir)
	if parseErr != nil {
		return "", "", parseErr
	}
	return strings.TrimSpace(docText), "", nil
}

func packageDocFromFile(path string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}
	if file == nil || file.Doc == nil {
		return "", nil
	}
	return strings.TrimSpace(file.Doc.Text()), nil
}

func packageDocForDir(dir, importPath string) (string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return "", err
	}
	if len(pkgs) == 0 {
		return "", nil
	}

	var parsedPkg *ast.Package
	for _, p := range pkgs {
		parsedPkg = p
		break
	}
	if parsedPkg == nil {
		return "", nil
	}
	pkgDoc := doc.New(parsedPkg, importPath, 0)
	if pkgDoc == nil {
		return "", nil
	}
	return strings.TrimSpace(pkgDoc.Doc), nil
}

func commandUsageText(root, relDir string) (string, error) {
	tempDir, err := os.MkdirTemp("", "docgen-cmd-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	binPath := filepath.Join(tempDir, slugify(relDir))
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer buildCancel()

	buildCmd := exec.CommandContext(buildCtx, "go", "build", "-o", binPath, "./"+relDir)
	buildCmd.Dir = root
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildCtx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(string(buildOut)), fmt.Errorf("timeout while building command")
	}
	if buildErr != nil {
		return strings.TrimSpace(string(buildOut)), buildErr
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer runCancel()

	runCmd := exec.CommandContext(runCtx, binPath, "-h")
	runCmd.Dir = root
	out, runErr := runCmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if runCtx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("timeout while capturing usage")
	}
	if runErr != nil && text == "" {
		return "", runErr
	}
	return text, nil
}

func parseCommandHelp(raw string) commandHelpDoc {
	doc := commandHelpDoc{
		Raw: strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n")),
	}
	if doc.Raw == "" {
		return doc
	}

	lines := strings.Split(doc.Raw, "\n")
	titleIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			titleIdx = i
			break
		}
	}
	if titleIdx < 0 {
		return doc
	}

	doc.Title = strings.TrimSpace(lines[titleIdx])
	if parts := strings.SplitN(doc.Title, " - ", 2); len(parts) == 2 {
		doc.Command = strings.TrimSpace(parts[0])
		doc.Synopsis = strings.TrimSpace(parts[1])
	}

	type sectionRef struct {
		key   string
		start int
		end   int
	}

	sections := make([]sectionRef, 0, 4)
	for i := titleIdx + 1; i < len(lines); i++ {
		if key := normalizeCommandHelpHeading(strings.TrimSpace(lines[i])); key != "" {
			sections = append(sections, sectionRef{
				key:   key,
				start: i,
			})
		}
	}
	for i := range sections {
		if i+1 < len(sections) {
			sections[i].end = sections[i+1].start
		} else {
			sections[i].end = len(lines)
		}
	}

	firstSection := len(lines)
	if len(sections) > 0 {
		firstSection = sections[0].start
	}
	for _, line := range lines[titleIdx+1 : firstSection] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		doc.Summary = append(doc.Summary, trimmed)
	}

	sectionStart := map[string]int{}
	sectionEnd := map[string]int{}
	for _, section := range sections {
		body := lines[section.start+1 : section.end]
		sectionStart[section.key] = section.start
		sectionEnd[section.key] = section.end
		switch section.key {
		case "usage":
			doc.Usage = parsePlainLines(body)
		case "subcommands":
			subcommandLines, flagLines := splitSubcommandAndFlagBlock(body)
			doc.Subcommands = parseCommandRows(subcommandLines)
			if len(doc.Flags) == 0 && len(flagLines) > 0 {
				doc.Flags = parseCommandFlags(flagLines)
			}
		case "exit_codes":
			doc.ExitCodes = parseCommandRows(body)
		case "flags":
			doc.Flags = parseCommandFlags(body)
		}
	}

	if len(doc.Flags) == 0 {
		start := titleIdx + 1
		if end, ok := sectionEnd["subcommands"]; ok {
			start = end
		} else if end, ok := sectionEnd["usage"]; ok {
			start = end
		}
		end := len(lines)
		if flagStart, ok := sectionStart["exit_codes"]; ok {
			end = flagStart
		}
		if start < end {
			doc.Flags = parseCommandFlags(lines[start:end])
		}
	}

	if len(doc.Summary) == 0 && doc.Synopsis != "" {
		doc.Summary = append(doc.Summary, doc.Synopsis)
	}

	return doc
}

func normalizeCommandHelpHeading(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasSuffix(line, ":") {
		return ""
	}
	line = strings.TrimSuffix(line, ":")
	line = strings.ToLower(strings.TrimSpace(line))
	switch line {
	case "usage":
		return "usage"
	case "subcommands":
		return "subcommands"
	case "flags", "options":
		return "flags"
	case "exit codes", "exit code":
		return "exit_codes"
	default:
		return ""
	}
}

func parsePlainLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func parseCommandRows(lines []string) []commandHelpRow {
	out := make([]commandHelpRow, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if commandFlagLineRegex.MatchString(line) {
			continue
		}
		if trimmed == "(none)" {
			out = append(out, commandHelpRow{Name: "(none)", Description: ""})
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		desc := strings.TrimSpace(trimmed[len(name):])
		if desc == "" && len(fields) > 1 {
			desc = strings.Join(fields[1:], " ")
		}
		out = append(out, commandHelpRow{
			Name:        name,
			Description: strings.TrimSpace(desc),
		})
	}
	return out
}

func splitSubcommandAndFlagBlock(lines []string) (subcommands []string, flags []string) {
	for i, line := range lines {
		if commandFlagLineRegex.MatchString(line) {
			return lines[:i], lines[i:]
		}
	}
	return lines, nil
}

func parseCommandFlags(lines []string) []commandHelpFlag {
	out := make([]commandHelpFlag, 0, 8)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		match := commandFlagLineRegex.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}

		flagName := "-" + strings.TrimSpace(match[1])
		flagType := strings.TrimSpace(match[2])
		descriptionLines := make([]string, 0, 2)
		for i+1 < len(lines) {
			next := lines[i+1]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" {
				i++
				if len(descriptionLines) > 0 {
					break
				}
				continue
			}
			if commandFlagLineRegex.MatchString(next) || normalizeCommandHelpHeading(nextTrimmed) != "" {
				break
			}
			descriptionLines = append(descriptionLines, nextTrimmed)
			i++
		}

		description := strings.TrimSpace(strings.Join(descriptionLines, " "))
		if description == "" {
			description = "-"
		}
		if flagType == "" {
			flagType = "-"
		}

		out = append(out, commandHelpFlag{
			Name:        flagName,
			Type:        flagType,
			Description: description,
		})
	}

	return out
}

func renderProgramReferenceBody(section, title, relPath, docText, sourceFile string) template.HTML {
	var md strings.Builder
	kind := "Program"
	switch section {
	case "apps":
		kind = "Application"
	case "services":
		kind = "Service"
	}

	docText = strings.TrimSpace(docText)
	if docText != "" {
		md.WriteString("## Overview\n\n")
		md.WriteString(docText)
		md.WriteString("\n\n")
	} else {
		md.WriteString("## Overview\n\n")
		md.WriteString("Documentation not available yet. Add package comments in `doc.go` or `docs.go`.\n\n")
	}

	md.WriteString("## Reference\n\n")
	md.WriteString("| Field | Value |\n")
	md.WriteString("| --- | --- |\n")
	md.WriteString("| Type | " + markdownCell(kind) + " |\n")
	md.WriteString("| Path | " + markdownInlineCode(relPath) + " |\n")
	if sourceFile != "" {
		md.WriteString("| Doc Source | " + markdownInlineCode(sourceFile) + " |\n")
	} else {
		md.WriteString("| Doc Source | package comments |\n")
	}

	return renderSectionMarkdownBody(title, relPath, md.String())
}

func renderCommandReferenceBody(title, relPath, docText, sourceFile string, help commandHelpDoc, usesFlags bool, captureErr string) template.HTML {
	var b strings.Builder
	docText = strings.TrimSpace(docText)
	commandName := strings.TrimSpace(help.Command)
	if commandName == "" {
		commandName = filepath.Base(relPath)
	}

	lead := strings.TrimSpace(help.Synopsis)
	if lead == "" && len(help.Summary) > 0 {
		lead = strings.TrimSpace(help.Summary[0])
	}
	if lead == "" {
		lead = "Command reference"
	}

	usagePrimary := ""
	if len(help.Usage) > 0 {
		usagePrimary = strings.TrimSpace(help.Usage[0])
	}
	if usagePrimary == "" {
		usagePrimary = commandName
	}

	docSource := "package comments"
	if sourceFile != "" {
		docSource = sourceFile
	}

	helpSource := "command does not use `flag` package"
	if usesFlags {
		helpSource = "flag.Usage via -h"
	}

	b.WriteString(`<section class="doc-ref-shell">`)
	b.WriteString(`<header class="doc-ref-hero">`)
	b.WriteString(`<h1>` + template.HTMLEscapeString(title) + `</h1>`)
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + `</p>`)
	b.WriteString(`<p class="doc-ref-lead">` + template.HTMLEscapeString(lead) + `</p>`)
	b.WriteString(`</header>`)

	b.WriteString(`<section class="doc-ref-grid">`)
	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Quick Start</h2>`)
	b.WriteString(`<pre class="doc-ref-code"><code>` + template.HTMLEscapeString(usagePrimary) + `</code></pre>`)
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>At A Glance</h2>`)
	b.WriteString(`<dl class="doc-ref-meta">`)
	b.WriteString(`<dt>Name</dt><dd><code>` + template.HTMLEscapeString(commandName) + `</code></dd>`)
	b.WriteString(`<dt>Help Source</dt><dd>` + template.HTMLEscapeString(helpSource) + `</dd>`)
	b.WriteString(`<dt>Doc Source</dt><dd>` + template.HTMLEscapeString(docSource) + `</dd>`)
	b.WriteString(`<dt>Flags</dt><dd>` + fmt.Sprintf("%d", len(help.Flags)) + `</dd>`)
	b.WriteString(`<dt>Subcommands</dt><dd>` + fmt.Sprintf("%d", countCommandRows(help.Subcommands)) + `</dd>`)
	b.WriteString(`</dl>`)
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)

	if docText != "" {
		b.WriteString(`<section class="doc-ref-card">`)
		b.WriteString(`<h2>Overview</h2>`)
		b.WriteString(`<div class="doc-markdown">`)
		b.WriteString(string(renderMarkdown(docText)))
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
	}

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Usage</h2>`)
	if len(help.Usage) > 0 {
		b.WriteString(`<ul class="doc-ref-list">`)
		for _, usage := range help.Usage {
			usage = strings.TrimSpace(usage)
			if usage == "" {
				continue
			}
			b.WriteString(`<li><code>` + template.HTMLEscapeString(usage) + `</code></li>`)
		}
		b.WriteString(`</ul>`)
	} else {
		b.WriteString(`<p class="doc-ref-empty">Usage information is not available.</p>`)
	}
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Subcommands</h2>`)
	if countCommandRows(help.Subcommands) > 0 {
		b.WriteString(`<table class="doc-ref-table">`)
		b.WriteString(`<thead><tr><th>Name</th><th>Description</th></tr></thead><tbody>`)
		for _, row := range help.Subcommands {
			if strings.TrimSpace(row.Name) == "" || row.Name == "(none)" {
				continue
			}
			description := strings.TrimSpace(row.Description)
			if description == "" {
				description = "-"
			}
			b.WriteString(`<tr><td><code>` + template.HTMLEscapeString(row.Name) + `</code></td><td>` + template.HTMLEscapeString(description) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	} else {
		b.WriteString(`<p class="doc-ref-empty">No subcommands.</p>`)
	}
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Flags</h2>`)
	if len(help.Flags) > 0 {
		b.WriteString(`<table class="doc-ref-table">`)
		b.WriteString(`<thead><tr><th>Flag</th><th>Type</th><th>Description</th></tr></thead><tbody>`)
		for _, option := range help.Flags {
			flagName := strings.TrimSpace(option.Name)
			if flagName == "" {
				continue
			}
			flagType := strings.TrimSpace(option.Type)
			if flagType == "" {
				flagType = "-"
			}
			description := strings.TrimSpace(option.Description)
			if description == "" {
				description = "-"
			}
			b.WriteString(`<tr><td><code>` + template.HTMLEscapeString(flagName) + `</code></td><td><code>` + template.HTMLEscapeString(flagType) + `</code></td><td>` + template.HTMLEscapeString(description) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	} else {
		b.WriteString(`<p class="doc-ref-empty">No flags documented.</p>`)
	}
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Exit Codes</h2>`)
	if len(help.ExitCodes) > 0 {
		b.WriteString(`<table class="doc-ref-table">`)
		b.WriteString(`<thead><tr><th>Code</th><th>Meaning</th></tr></thead><tbody>`)
		for _, row := range help.ExitCodes {
			code := strings.TrimSpace(row.Name)
			if code == "" {
				continue
			}
			meaning := strings.TrimSpace(row.Description)
			if meaning == "" {
				meaning = "-"
			}
			b.WriteString(`<tr><td><code>` + template.HTMLEscapeString(code) + `</code></td><td>` + template.HTMLEscapeString(meaning) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	} else {
		b.WriteString(`<p class="doc-ref-empty">No exit code table provided.</p>`)
	}
	b.WriteString(`</section>`)

	if captureErr != "" {
		b.WriteString(`<p class="doc-ref-callout">Help capture warning: ` + template.HTMLEscapeString(captureErr) + `</p>`)
	}
	if !usesFlags && docText == "" {
		b.WriteString(`<p class="doc-ref-callout">This command has no flag usage output and no package-level documentation yet.</p>`)
	}
	if len(help.Usage) == 0 && len(help.Subcommands) == 0 && len(help.Flags) == 0 && strings.TrimSpace(help.Raw) != "" {
		b.WriteString(`<section class="doc-ref-card">`)
		b.WriteString(`<h2>Raw Help Output</h2>`)
		b.WriteString(`<pre class="doc-ref-code"><code>` + template.HTMLEscapeString(help.Raw) + `</code></pre>`)
		b.WriteString(`</section>`)
	}

	b.WriteString(`</section>`)
	return template.HTML(b.String())
}

func renderAppReferenceBody(title, relPath, docText, sourceFile, previewPath string, help commandHelpDoc, usesFlags bool, captureErr string) template.HTML {
	var b strings.Builder
	docText = strings.TrimSpace(docText)
	if docText == "" {
		docText = "Documentation not available yet. Add package comments in `doc.go` or `docs.go`."
	}
	docSource := "package comments"
	if sourceFile != "" {
		docSource = sourceFile
	}

	lead := firstLine(docText)
	if lead == "" {
		lead = "Application reference"
	}

	b.WriteString(`<section class="doc-ref-shell">`)
	b.WriteString(`<header class="doc-ref-hero">`)
	b.WriteString(`<h1>` + template.HTMLEscapeString(title) + `</h1>`)
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + `</p>`)
	b.WriteString(`<p class="doc-ref-lead">` + template.HTMLEscapeString(lead) + `</p>`)
	b.WriteString(`</header>`)

	b.WriteString(`<section class="doc-ref-grid">`)
	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>At A Glance</h2>`)
	b.WriteString(`<dl class="doc-ref-meta">`)
	b.WriteString(`<dt>Type</dt><dd>Application</dd>`)
	b.WriteString(`<dt>Path</dt><dd><code>` + template.HTMLEscapeString(relPath) + `</code></dd>`)
	b.WriteString(`<dt>Doc Source</dt><dd>` + template.HTMLEscapeString(docSource) + `</dd>`)
	if previewPath != "" {
		b.WriteString(`<dt>Preview</dt><dd>Available</dd>`)
	} else {
		b.WriteString(`<dt>Preview</dt><dd>Not provided</dd>`)
	}
	b.WriteString(`</dl>`)
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	if previewPath != "" {
		b.WriteString(`<h2>Preview</h2>`)
		b.WriteString(`<figure class="doc-ref-preview">`)
		b.WriteString(`<img src="` + template.HTMLEscapeString(previewPath) + `" alt="` + template.HTMLEscapeString(title) + ` preview" />`)
		b.WriteString(`<figcaption>` + template.HTMLEscapeString(title) + ` preview image</figcaption>`)
		b.WriteString(`</figure>`)
	} else {
		b.WriteString(`<h2>Preview</h2>`)
		b.WriteString(`<p class="doc-ref-empty">Add <code>` + template.HTMLEscapeString(relPath) + `/preview.png</code> to show an app screenshot here.</p>`)
	}
	b.WriteString(`</section>`)
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Description</h2>`)
	b.WriteString(`<div class="doc-markdown">`)
	b.WriteString(string(renderMarkdown(docText)))
	b.WriteString(`</div>`)
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Flags</h2>`)
	if len(help.Flags) > 0 {
		b.WriteString(`<table class="doc-ref-table">`)
		b.WriteString(`<thead><tr><th>Flag</th><th>Type</th><th>Description</th></tr></thead><tbody>`)
		for _, option := range help.Flags {
			flagName := strings.TrimSpace(option.Name)
			if flagName == "" {
				continue
			}
			flagType := strings.TrimSpace(option.Type)
			if flagType == "" {
				flagType = "-"
			}
			desc := strings.TrimSpace(option.Description)
			if desc == "" {
				desc = "-"
			}
			b.WriteString(`<tr><td><code>` + template.HTMLEscapeString(flagName) + `</code></td><td><code>` + template.HTMLEscapeString(flagType) + `</code></td><td>` + template.HTMLEscapeString(desc) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	} else {
		msg := "This app does not expose `flag.Usage` output."
		if usesFlags {
			msg = "No flags documented by app help output."
		}
		b.WriteString(`<p class="doc-ref-empty">` + template.HTMLEscapeString(msg) + `</p>`)
	}
	b.WriteString(`</section>`)

	b.WriteString(`<section class="doc-ref-card">`)
	b.WriteString(`<h2>Subcommands</h2>`)
	if countCommandRows(help.Subcommands) > 0 {
		b.WriteString(`<table class="doc-ref-table">`)
		b.WriteString(`<thead><tr><th>Name</th><th>Description</th></tr></thead><tbody>`)
		for _, row := range help.Subcommands {
			if strings.TrimSpace(row.Name) == "" || row.Name == "(none)" {
				continue
			}
			desc := strings.TrimSpace(row.Description)
			if desc == "" {
				desc = "-"
			}
			b.WriteString(`<tr><td><code>` + template.HTMLEscapeString(row.Name) + `</code></td><td>` + template.HTMLEscapeString(desc) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	} else {
		b.WriteString(`<p class="doc-ref-empty">No subcommands.</p>`)
	}
	b.WriteString(`</section>`)

	if captureErr != "" {
		b.WriteString(`<p class="doc-ref-callout">Help capture warning: ` + template.HTMLEscapeString(captureErr) + `</p>`)
	}

	b.WriteString(`</section>`)
	return template.HTML(b.String())
}

func firstLine(value string) string {
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "#*- ")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func countCommandRows(rows []commandHelpRow) int {
	count := 0
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" || row.Name == "(none)" {
			continue
		}
		count++
	}
	return count
}

func renderMarkdownCards(source string) template.HTML {
	sections := splitMarkdownSections(source)
	if len(sections) == 0 {
		return template.HTML(`<section class="doc-card-stack"><section class="doc-section-card"><div class="doc-markdown"><p>No content.</p></div></section></section>`)
	}

	var b strings.Builder
	b.WriteString(`<section class="doc-card-stack">`)
	for _, section := range sections {
		b.WriteString(`<section class="doc-section-card">`)
		if strings.TrimSpace(section.Title) != "" {
			b.WriteString(`<h2>` + template.HTMLEscapeString(section.Title) + `</h2>`)
		}
		b.WriteString(`<div class="doc-markdown">`)
		b.WriteString(string(renderMarkdown(section.Content)))
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String())
}

func splitMarkdownSections(source string) []markdownSection {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(source, "\n")

	currentTitle := "Overview"
	body := make([]string, 0, 32)
	out := make([]markdownSection, 0, 8)
	seenH2 := false

	flush := func() {
		content := strings.TrimSpace(strings.Join(body, "\n"))
		if content == "" {
			body = body[:0]
			return
		}
		out = append(out, markdownSection{
			Title:   strings.TrimSpace(currentTitle),
			Content: content,
		})
		body = body[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			// Top heading is already rendered separately by docgen templates.
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			currentTitle = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if currentTitle == "" {
				currentTitle = "Section"
			}
			seenH2 = true
			continue
		}
		body = append(body, line)
	}
	flush()

	if len(out) == 0 {
		content := strings.TrimSpace(source)
		if content == "" {
			return nil
		}
		return []markdownSection{{
			Title:   "Overview",
			Content: content,
		}}
	}

	if !seenH2 && len(out) == 1 && strings.TrimSpace(out[0].Title) == "" {
		out[0].Title = "Overview"
	}
	return out
}

func renderSectionMarkdownBody(title, relPath, markdown string) template.HTML {
	var b strings.Builder
	b.WriteString("<h1>" + template.HTMLEscapeString(title) + "</h1>\n")
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + "</p>\n")
	b.WriteString(`<section class="doc-markdown">`)
	b.WriteString(string(renderMarkdown(markdown)))
	b.WriteString("</section>")
	return template.HTML(b.String())
}

func renderAPIJSONBody(relPath string, spec apiSchema) template.HTML {
	title := strings.TrimSpace(spec.Service.Name)
	if title == "" {
		title = filepath.Base(filepath.Dir(relPath))
	}

	var b strings.Builder
	serviceDescription := strings.TrimSpace(spec.Service.Description)
	lead := serviceDescription
	if lead == "" {
		lead = "API reference for " + title
	}

	b.WriteString(`<section class="doc-ref-shell doc-api-shell">`)
	b.WriteString(`<header class="doc-ref-hero">`)
	b.WriteString(`<h1>` + template.HTMLEscapeString(title) + `</h1>`)
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + `</p>`)
	b.WriteString(`<p class="doc-ref-lead">` + template.HTMLEscapeString(lead) + `</p>`)
	b.WriteString(`</header>`)

	b.WriteString(`<section class="doc-section-card">`)
	b.WriteString(`<h2>Service</h2>`)
	b.WriteString(`<table class="doc-ref-table">`)
	b.WriteString(`<thead><tr><th>Field</th><th>Value</th></tr></thead><tbody>`)
	b.WriteString(`<tr><td>Name</td><td><code>` + template.HTMLEscapeString(spec.Service.Name) + `</code></td></tr>`)
	b.WriteString(`<tr><td>Package</td><td><code>` + template.HTMLEscapeString(spec.Service.Package) + `</code></td></tr>`)
	b.WriteString(`<tr><td>Service ID</td><td><code>` + template.HTMLEscapeString(string(spec.Service.ID)) + `</code></td></tr>`)
	if len(spec.Imports) > 0 {
		b.WriteString(`<tr><td>Imports</td><td><code>` + template.HTMLEscapeString(strings.Join(spec.Imports, ", ")) + `</code></td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	if serviceDescription != "" {
		b.WriteString(`<div class="doc-markdown">`)
		b.WriteString(string(renderMarkdown(serviceDescription)))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)

	if len(spec.Types) > 0 {
		b.WriteString(`<section class="doc-section-card">`)
		b.WriteString(`<h2>Types</h2>`)
		b.WriteString(`<div class="api-card-list">`)
		for _, t := range spec.Types {
			b.WriteString(`<article class="api-item-card">`)
			b.WriteString(`<h3>` + template.HTMLEscapeString(t.Name) + `</h3>`)
			if desc := strings.TrimSpace(t.Description); desc != "" {
				b.WriteString(`<div class="doc-markdown">` + string(renderMarkdown(desc)) + `</div>`)
			}
			if len(t.Fields) == 0 {
				b.WriteString(`<p class="doc-ref-empty">No fields.</p>`)
			} else {
				b.WriteString(`<table class="doc-ref-table">`)
				b.WriteString(`<thead><tr><th>Field</th><th>Type</th><th>Description</th></tr></thead><tbody>`)
				for _, field := range t.Fields {
					desc := strings.TrimSpace(field.Description)
					if desc == "" {
						desc = "-"
					}
					b.WriteString(`<tr><td><code>` + template.HTMLEscapeString(field.Name) + `</code></td><td><code>` + template.HTMLEscapeString(field.Type) + `</code></td><td>` + template.HTMLEscapeString(desc) + `</td></tr>`)
				}
				b.WriteString(`</tbody></table>`)
			}
			b.WriteString(`</article>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
	}

	if len(spec.Requests) > 0 {
		b.WriteString(`<section class="doc-section-card">`)
		b.WriteString(`<h2>Requests</h2>`)
		b.WriteString(`<div class="api-card-list">`)
		for _, req := range spec.Requests {
			b.WriteString(`<article class="api-item-card">`)
			b.WriteString(`<header class="api-item-head">`)
			b.WriteString(`<h3>` + template.HTMLEscapeString(req.Name) + `</h3>`)
			b.WriteString(`<p class="api-item-id"><code>` + template.HTMLEscapeString(string(req.ID)) + `</code></p>`)
			b.WriteString(`</header>`)
			if desc := strings.TrimSpace(req.Description); desc != "" {
				b.WriteString(`<div class="doc-markdown">` + string(renderMarkdown(desc)) + `</div>`)
			}
			b.WriteString(`<div class="api-io-grid">`)
			b.WriteString(`<section class="api-io-card">`)
			b.WriteString(`<h4>Input</h4>`)
			b.WriteString(`<p><strong>Type:</strong> <code>` + template.HTMLEscapeString(methodPayloadType(req)) + `</code></p>`)
			reqDesc := strings.TrimSpace(req.RequestDescription)
			if reqDesc == "" {
				reqDesc = "No input description."
			}
			b.WriteString(`<div class="doc-markdown">` + string(renderMarkdown(reqDesc)) + `</div>`)
			b.WriteString(`</section>`)
			b.WriteString(`<section class="api-io-card">`)
			b.WriteString(`<h4>Output</h4>`)
			respType := strings.TrimSpace(req.ResponseType)
			if req.OneWay {
				respType = "none (one-way)"
			}
			if respType == "" {
				respType = "none"
			}
			b.WriteString(`<p><strong>Type:</strong> <code>` + template.HTMLEscapeString(respType) + `</code></p>`)
			respDesc := strings.TrimSpace(req.ResponseDescription)
			if req.OneWay && respDesc == "" {
				respDesc = "No response payload for one-way request."
			}
			if !req.OneWay && respDesc == "" {
				respDesc = "No output description."
			}
			b.WriteString(`<div class="doc-markdown">` + string(renderMarkdown(respDesc)) + `</div>`)
			b.WriteString(`</section>`)
			b.WriteString(`</div>`)
			b.WriteString(`</article>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
	}

	if len(spec.Events) > 0 {
		b.WriteString(`<section class="doc-section-card">`)
		b.WriteString(`<h2>Events</h2>`)
		b.WriteString(`<div class="api-card-list">`)
		for _, ev := range spec.Events {
			b.WriteString(`<article class="api-item-card">`)
			b.WriteString(`<header class="api-item-head">`)
			b.WriteString(`<h3>` + template.HTMLEscapeString(ev.Name) + `</h3>`)
			b.WriteString(`<p class="api-item-id"><code>` + template.HTMLEscapeString(string(ev.ID)) + `</code></p>`)
			b.WriteString(`</header>`)
			if desc := strings.TrimSpace(ev.Description); desc != "" {
				b.WriteString(`<div class="doc-markdown">` + string(renderMarkdown(desc)) + `</div>`)
			}
			b.WriteString(`<div class="api-io-grid">`)
			b.WriteString(`<section class="api-io-card">`)
			b.WriteString(`<h4>Payload</h4>`)
			b.WriteString(`<p><strong>Type:</strong> <code>` + template.HTMLEscapeString(methodPayloadType(ev)) + `</code></p>`)
			payloadDesc := strings.TrimSpace(ev.RequestDescription)
			if payloadDesc == "" {
				payloadDesc = "No payload description."
			}
			b.WriteString(`<div class="doc-markdown">` + string(renderMarkdown(payloadDesc)) + `</div>`)
			b.WriteString(`</section>`)
			b.WriteString(`</div>`)
			b.WriteString(`</article>`)
		}
		b.WriteString(`</div>`)
		b.WriteString(`</section>`)
	}

	b.WriteString(`</section>`)
	return template.HTML(b.String())
}

func methodPayloadType(method apiSchemaMethod) string {
	if value := strings.TrimSpace(method.RequestType); value != "" {
		return value
	}
	return strings.TrimSpace(method.PayloadType)
}

func markdownInlineCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "`", "\\`")
	return "`" + value + "`"
}

func markdownCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func renderProjectBody(title, relPath, source string) template.HTML {
	var b strings.Builder
	b.WriteString("<h1>" + template.HTMLEscapeString(title) + "</h1>\n")
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + "</p>\n")
	b.WriteString(string(renderMarkdownCards(source)))
	return template.HTML(b.String())
}

func renderAPIBody(d apiDoc) template.HTML {
	return d.BodyHTML
}

func renderPackageBody(pkg apiPackage) (template.HTML, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkg.Dir, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return "", err
	}
	if len(pkgs) == 0 {
		return "", errors.New("no non-test go files")
	}

	var parsedPkg *ast.Package
	for _, p := range pkgs {
		parsedPkg = p
		break
	}
	if parsedPkg == nil {
		return "", errors.New("unable to parse package")
	}

	pkgDoc := doc.New(parsedPkg, pkg.ImportPath, 0)
	if pkgDoc == nil {
		return "", errors.New("unable to build package documentation")
	}

	var b strings.Builder
	b.WriteString(`<section class="doc-api">`)
	b.WriteString("<h1>" + template.HTMLEscapeString(pkg.ImportPath) + "</h1>\n")
	b.WriteString(`<p class="muted mono doc-path">package ` + template.HTMLEscapeString(pkgDoc.Name) + "</p>\n")
	var overview strings.Builder
	overview.WriteString("## Package Overview\n\n")
	if strings.TrimSpace(pkgDoc.Doc) != "" {
		overview.WriteString(strings.TrimSpace(pkgDoc.Doc))
		overview.WriteString("\n\n")
	} else {
		overview.WriteString("No package-level documentation is provided.\n\n")
	}
	overview.WriteString("| Export Group | Count |\n")
	overview.WriteString("| --- | --- |\n")
	overview.WriteString(fmt.Sprintf("| Constants | %d |\n", len(pkgDoc.Consts)))
	overview.WriteString(fmt.Sprintf("| Variables | %d |\n", len(pkgDoc.Vars)))
	overview.WriteString(fmt.Sprintf("| Functions | %d |\n", len(pkgDoc.Funcs)))
	overview.WriteString(fmt.Sprintf("| Types | %d |\n", len(pkgDoc.Types)))
	b.WriteString(`<section class="doc-markdown">`)
	b.WriteString(string(renderMarkdown(overview.String())))
	b.WriteString(`</section>`)

	appendValueSection(&b, "Constants", pkgDoc.Consts, fset)
	appendValueSection(&b, "Variables", pkgDoc.Vars, fset)
	appendFuncSection(&b, "Functions", pkgDoc.Funcs, fset)
	appendTypeSection(&b, pkgDoc.Types, fset)
	b.WriteString("</section>")
	return template.HTML(b.String()), nil
}

func appendValueSection(b *strings.Builder, heading string, values []*doc.Value, fset *token.FileSet) {
	if len(values) == 0 {
		return
	}

	b.WriteString("<h2>" + template.HTMLEscapeString(heading) + "</h2>")
	for _, value := range values {
		if decl := formattedDecl(fset, value.Decl); decl != "" {
			b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
		}
		if strings.TrimSpace(value.Doc) != "" {
			b.WriteString(`<section class="doc-markdown">`)
			b.WriteString(string(renderMarkdown(value.Doc)))
			b.WriteString(`</section>`)
		}
	}
}

func appendFuncSection(b *strings.Builder, heading string, funcs []*doc.Func, fset *token.FileSet) {
	if len(funcs) == 0 {
		return
	}

	b.WriteString("<h2>" + template.HTMLEscapeString(heading) + "</h2>")
	for _, fn := range funcs {
		if decl := formattedDecl(fset, fn.Decl); decl != "" {
			b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
		}
		if strings.TrimSpace(fn.Doc) != "" {
			b.WriteString(`<section class="doc-markdown">`)
			b.WriteString(string(renderMarkdown(fn.Doc)))
			b.WriteString(`</section>`)
		}
	}
}

func appendTypeSection(b *strings.Builder, types []*doc.Type, fset *token.FileSet) {
	if len(types) == 0 {
		return
	}

	b.WriteString("<h2>Types</h2>")
	for _, t := range types {
		b.WriteString("<h3>" + template.HTMLEscapeString(t.Name) + "</h3>")
		if decl := formattedDecl(fset, t.Decl); decl != "" {
			b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
		}
		if strings.TrimSpace(t.Doc) != "" {
			b.WriteString(`<section class="doc-markdown">`)
			b.WriteString(string(renderMarkdown(t.Doc)))
			b.WriteString(`</section>`)
		}

		appendTypeValues(b, "Constants", t.Consts, fset)
		appendTypeValues(b, "Variables", t.Vars, fset)
		appendTypeFuncs(b, "Functions", t.Funcs, fset)
		appendTypeFuncs(b, "Methods", t.Methods, fset)
	}
}

func appendTypeValues(b *strings.Builder, heading string, values []*doc.Value, fset *token.FileSet) {
	if len(values) == 0 {
		return
	}

	b.WriteString("<h4>" + template.HTMLEscapeString(heading) + "</h4>")
	for _, value := range values {
		if decl := formattedDecl(fset, value.Decl); decl != "" {
			b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
		}
		if strings.TrimSpace(value.Doc) != "" {
			b.WriteString(`<section class="doc-markdown">`)
			b.WriteString(string(renderMarkdown(value.Doc)))
			b.WriteString(`</section>`)
		}
	}
}

func appendTypeFuncs(b *strings.Builder, heading string, funcs []*doc.Func, fset *token.FileSet) {
	if len(funcs) == 0 {
		return
	}

	b.WriteString("<h4>" + template.HTMLEscapeString(heading) + "</h4>")
	for _, fn := range funcs {
		if decl := formattedDecl(fset, fn.Decl); decl != "" {
			b.WriteString("<pre><code>" + template.HTMLEscapeString(decl) + "</code></pre>")
		}
		if strings.TrimSpace(fn.Doc) != "" {
			b.WriteString(`<section class="doc-markdown">`)
			b.WriteString(string(renderMarkdown(fn.Doc)))
			b.WriteString(`</section>`)
		}
	}
}

func formattedDecl(fset *token.FileSet, node any) string {
	if node == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func renderProjectIndexBody(projectDocs []markdownDoc) template.HTML {
	var b strings.Builder
	b.WriteString("<h1>Project Documentation</h1>\n")
	b.WriteString(`<p class="muted">Index of project guides and design documents.</p>`)
	b.WriteString(`<ul class="doc-list">`)
	for _, d := range projectDocs {
		if d.RelPath == "README.md" {
			continue
		}
		b.WriteString(`<li><a href="` + template.HTMLEscapeString(d.HTMLFile) + `">` + template.HTMLEscapeString(d.Title) + "</a></li>")
	}
	b.WriteString("</ul>")
	return template.HTML(b.String())
}

func renderReadmeIndexBody(relPath, source string) template.HTML {
	var b strings.Builder
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + "</p>\n")
	b.WriteString(string(renderMarkdownCards(source)))
	return template.HTML(b.String())
}

func renderMarkdown(source string) template.HTML {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(source, "\n")

	var b strings.Builder
	var paragraph []string
	var listItems []string
	listType := ""
	inCode := false
	inHTMLBlock := false
	htmlBlockTag := ""

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		content := renderInline(strings.Join(paragraph, " "))
		b.WriteString("<p>" + content + "</p>\n")
		paragraph = paragraph[:0]
	}

	flushList := func() {
		if len(listItems) == 0 || listType == "" {
			return
		}
		b.WriteString("<" + listType + ">\n")
		for _, item := range listItems {
			b.WriteString("<li>" + renderInline(item) + "</li>\n")
		}
		b.WriteString("</" + listType + ">\n")
		listItems = listItems[:0]
		listType = ""
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)

		if inCode {
			if strings.HasPrefix(trimmed, "```") {
				b.WriteString("</code></pre>\n")
				inCode = false
				continue
			}
			b.WriteString(template.HTMLEscapeString(line) + "\n")
			continue
		}

		if inHTMLBlock {
			b.WriteString(line + "\n")
			if closesHTMLBlock(trimmed, htmlBlockTag) {
				inHTMLBlock = false
				htmlBlockTag = ""
			}
			continue
		}

		if trimmed == "" {
			flushParagraph()
			flushList()
			continue
		}

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			flushList()
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			if lang == "" {
				b.WriteString("<pre><code>")
			} else {
				b.WriteString(`<pre><code class="language-` + template.HTMLEscapeString(lang) + `">`)
			}
			inCode = true
			continue
		}

		if tag, isStart, selfClosing := htmlBlockStartTag(trimmed); isStart {
			flushParagraph()
			flushList()
			b.WriteString(line + "\n")
			if !selfClosing && !isInlineHTMLTag(tag) && !closesHTMLBlock(trimmed, tag) {
				inHTMLBlock = true
				htmlBlockTag = tag
			}
			continue
		}

		if isRawHTMLLine(trimmed) {
			flushParagraph()
			flushList()
			b.WriteString(line + "\n")
			continue
		}

		if headingMatch := mdHeadingPattern.FindStringSubmatch(trimmed); len(headingMatch) == 3 {
			flushParagraph()
			flushList()
			level := len(headingMatch[1])
			if level < 1 {
				level = 1
			}
			if level > 6 {
				level = 6
			}
			content := renderInline(strings.TrimSpace(headingMatch[2]))
			b.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, content, level))
			continue
		}

		if isHorizontalRule(trimmed) {
			flushParagraph()
			flushList()
			b.WriteString("<hr />\n")
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			flushList()
			var quoteLines []string
			for ; i < len(lines); i++ {
				quoteLine := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(quoteLine, ">") {
					i--
					break
				}
				quoteLines = append(quoteLines, strings.TrimSpace(strings.TrimPrefix(quoteLine, ">")))
			}
			b.WriteString("<blockquote><p>" + renderInline(strings.Join(quoteLines, " ")) + "</p></blockquote>\n")
			continue
		}

		if strings.Contains(trimmed, "|") &&
			i+1 < len(lines) &&
			mdTableDividerPattern.MatchString(strings.TrimSpace(lines[i+1])) {
			flushParagraph()
			flushList()

			headers := parseTableCells(trimmed)
			if len(headers) == 0 {
				continue
			}

			b.WriteString("<table>\n<thead><tr>")
			for _, cell := range headers {
				b.WriteString("<th>" + renderInline(cell) + "</th>")
			}
			b.WriteString("</tr></thead>\n<tbody>\n")

			i++
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if next == "" || !strings.Contains(next, "|") {
					break
				}
				i++
				rowCells := parseTableCells(next)
				b.WriteString("<tr>")
				for c := 0; c < len(headers); c++ {
					cell := ""
					if c < len(rowCells) {
						cell = rowCells[c]
					}
					b.WriteString("<td>" + renderInline(cell) + "</td>")
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</tbody>\n</table>\n")
			continue
		}

		if listMatch := mdUnorderedPattern.FindStringSubmatch(trimmed); len(listMatch) == 2 {
			flushParagraph()
			if listType != "ul" {
				flushList()
				listType = "ul"
			}
			listItems = append(listItems, listMatch[1])
			continue
		}

		if listMatch := mdOrderedPattern.FindStringSubmatch(trimmed); len(listMatch) == 2 {
			flushParagraph()
			if listType != "ol" {
				flushList()
				listType = "ol"
			}
			listItems = append(listItems, listMatch[1])
			continue
		}

		if listType != "" {
			flushList()
		}
		paragraph = append(paragraph, trimmed)
	}

	flushParagraph()
	flushList()
	if inCode {
		b.WriteString("</code></pre>\n")
	}

	return template.HTML(b.String())
}

func renderInline(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	escaped := template.HTMLEscapeString(text)
	placeholders := make([]string, 0, 4)

	escaped = mdCodePattern.ReplaceAllStringFunc(escaped, func(match string) string {
		sub := mdCodePattern.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		token := fmt.Sprintf("{{{CODE_%d}}}", len(placeholders))
		placeholders = append(placeholders, "<code>"+sub[1]+"</code>")
		return token
	})

	escaped = mdImagePattern.ReplaceAllStringFunc(escaped, func(match string) string {
		sub := mdImagePattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		alt := strings.TrimSpace(sub[1])
		src := strings.TrimSpace(sub[2])
		if src == "" {
			return match
		}
		return `<img src="` + src + `" alt="` + alt + `" />`
	})

	escaped = mdLinkPattern.ReplaceAllStringFunc(escaped, func(match string) string {
		sub := mdLinkPattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		label := strings.TrimSpace(sub[1])
		href := strings.TrimSpace(sub[2])
		if href == "" {
			return label
		}
		return `<a href="` + href + `">` + label + `</a>`
	})

	escaped = mdBoldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = mdItalicPattern.ReplaceAllString(escaped, "<em>$1</em>")

	for i, code := range placeholders {
		escaped = strings.ReplaceAll(escaped, fmt.Sprintf("{{{CODE_%d}}}", i), code)
	}
	return escaped
}

func parseTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func isRawHTMLLine(line string) bool {
	if line == "" || !strings.HasPrefix(line, "<") {
		return false
	}
	if strings.HasPrefix(line, "<!--") {
		return true
	}
	return strings.HasSuffix(line, ">")
}

func isHorizontalRule(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	line = strings.ReplaceAll(line, " ", "")
	if len(line) < 3 {
		return false
	}
	switch {
	case strings.Trim(line, "-") == "":
		return true
	case strings.Trim(line, "*") == "":
		return true
	case strings.Trim(line, "_") == "":
		return true
	default:
		return false
	}
}

func htmlBlockStartTag(line string) (tag string, isStart bool, selfClosing bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false, false
	}
	if strings.HasPrefix(line, "<!--") {
		return "!--", true, strings.Contains(line, "-->")
	}
	if strings.HasPrefix(line, "<?") || strings.HasPrefix(strings.ToLower(line), "<!doctype") {
		return "", true, true
	}
	if m := htmlSelfTagPattern.FindStringSubmatch(line); len(m) == 3 {
		return strings.ToLower(m[1]), true, true
	}
	if strings.HasPrefix(line, "</") {
		return "", false, false
	}
	if m := htmlOpenTagPattern.FindStringSubmatch(line); len(m) == 3 {
		return strings.ToLower(m[1]), true, false
	}
	return "", false, false
}

func closesHTMLBlock(line, tag string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if tag == "!--" {
		return strings.Contains(line, "-->")
	}
	if strings.Contains(strings.ToLower(line), "</"+strings.ToLower(tag)+">") {
		return true
	}
	if m := htmlCloseTagPattern.FindStringSubmatch(line); len(m) == 2 {
		return strings.EqualFold(m[1], tag)
	}
	return false
}

func isInlineHTMLTag(tag string) bool {
	switch strings.ToLower(tag) {
	case "a", "abbr", "b", "br", "code", "em", "i", "img", "kbd", "mark", "q", "s", "small", "span", "strong", "sub", "sup", "time", "u", "var":
		return true
	default:
		return false
	}
}

func rewriteDocLinks(source, currentRel string, markdownMap map[string]string) string {
	rewritten := markdownLinkPattern.ReplaceAllStringFunc(source, func(match string) string {
		sub := markdownLinkPattern.FindStringSubmatch(match)
		if len(sub) != 4 {
			return match
		}
		targetPath, suffix := splitLinkTarget(sub[2])
		newPath := rewriteLinkTarget(targetPath, currentRel, markdownMap)
		return sub[1] + newPath + suffix + sub[3]
	})

	rewritten = htmlAttrLinkPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		sub := htmlAttrLinkPattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		newPath := rewriteLinkTarget(sub[2], currentRel, markdownMap)
		return sub[1] + `="` + newPath + `"`
	})

	return rewritten
}

func splitLinkTarget(raw string) (targetPath, suffix string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	for i, r := range raw {
		if unicode.IsSpace(r) {
			return raw[:i], raw[i:]
		}
	}
	return raw, ""
}

func rewriteLinkTarget(target, currentRel string, markdownMap map[string]string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return target
	}
	if strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#") {
		return target
	}

	wrapped := strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">")
	if wrapped {
		target = strings.TrimPrefix(strings.TrimSuffix(target, ">"), "<")
	}

	fragment := ""
	if idx := strings.Index(target, "#"); idx >= 0 {
		fragment = target[idx:]
		target = target[:idx]
	}

	target = strings.TrimPrefix(target, "./")
	target = strings.TrimPrefix(target, "/")

	resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(currentRel), target)))

	if newPath, ok := markdownMap[resolved]; ok {
		out := newPath + fragment
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}

	if strings.HasPrefix(resolved, "docs/assets/") {
		out := "assets/" + strings.TrimPrefix(resolved, "docs/assets/") + fragment
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}

	if resolved == "data/icons/logo/logo.png" {
		out := "assets/logo.png" + fragment
		if wrapped {
			return "<" + out + ">"
		}
		return out
	}

	out := target + fragment
	if wrapped {
		return "<" + out + ">"
	}
	return out
}

func buildHead(title, description string) template.HTML {
	head := fallbackHead

	head = scriptTagPattern.ReplaceAllString(head, "")
	head = stylesheetLinkPattern.ReplaceAllString(head, "")

	titleTag := "<title>" + template.HTMLEscapeString(title) + "</title>"
	if titleTagPattern.MatchString(head) {
		head = titleTagPattern.ReplaceAllString(head, titleTag)
	} else {
		head = insertBeforeHeadClose(head, titleTag)
	}

	descTag := `<meta name="description" content="` + template.HTMLEscapeString(description) + `" />`
	if descriptionMetaTag.MatchString(head) {
		head = descriptionMetaTag.ReplaceAllString(head, descTag)
	} else {
		head = insertBeforeHeadClose(head, descTag)
	}

	head = insertBeforeHeadClose(head, `<link rel="stylesheet" href="styles.css" />`)
	head = insertBeforeHeadClose(head, `<link rel="stylesheet" href="docgen.css" />`)
	return template.HTML(head)
}

func insertBeforeHeadClose(head, insert string) string {
	lower := strings.ToLower(head)
	idx := strings.LastIndex(lower, "</head>")
	if idx < 0 {
		return head + "\n" + insert
	}
	return head[:idx] + "  " + insert + "\n" + head[idx:]
}

func parseDocsIndex(projectDocs []markdownDoc) docsIndexMeta {
	meta := docsIndexMeta{
		Order:  map[string]int{},
		Titles: map[string]string{},
	}

	indexDoc, ok := findProjectDoc(projectDocs, "docs/index.md")
	if !ok {
		return meta
	}

	order := 0
	for _, line := range strings.Split(indexDoc.Source, "\n") {
		matches := mdLinkPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) != 3 {
				continue
			}
			title := strings.TrimSpace(match[1])
			target := strings.TrimSpace(match[2])
			targetPath, _ := splitLinkTarget(target)
			relPath, ok := resolveMarkdownRelPath(indexDoc.RelPath, targetPath)
			if !ok {
				continue
			}
			if strings.ToLower(filepath.Ext(relPath)) != ".md" {
				continue
			}
			if _, exists := meta.Order[relPath]; !exists {
				meta.Order[relPath] = order
				order++
			}
			if title != "" {
				meta.Titles[relPath] = title
			}
		}
	}

	return meta
}

func buildSidebar(projectDocs []markdownDoc, apiDocs []apiDoc, docsIndex docsIndexMeta, homeRelPath, active string) []navSection {
	var sections []navSection

	if docsSection := buildDocsSection(projectDocs, docsIndex, homeRelPath, active); len(docsSection.Groups) > 0 {
		sections = append(sections, docsSection)
	}

	for _, spec := range []struct {
		title   string
		section string
	}{
		{title: "Apps", section: "apps"},
		{title: "Commands", section: "cmd"},
		{title: "Services", section: "services"},
		{title: "API", section: "api"},
		{title: "Packages", section: "pkg"},
	} {
		navSection := buildGoSection(spec.title, spec.section, apiDocs, active)
		if len(navSection.Groups) > 0 {
			sections = append(sections, navSection)
		}
	}

	return sections
}

func buildDocsSection(projectDocs []markdownDoc, docsIndex docsIndexMeta, homeRelPath, active string) navSection {
	type rankedEntry struct {
		entry navEntry
		order int
	}

	groups := map[string][]rankedEntry{}
	for i, d := range projectDocs {
		if d.RelPath == homeRelPath || d.RelPath == "README.md" {
			continue
		}
		if !strings.HasPrefix(d.RelPath, "docs/") {
			continue
		}

		title := strings.TrimSpace(d.Title)
		if override, ok := docsIndex.Titles[d.RelPath]; ok && strings.TrimSpace(override) != "" {
			title = strings.TrimSpace(override)
		}
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(d.RelPath), filepath.Ext(d.RelPath))
		}

		sortOrder := 100000 + i
		if order, ok := docsIndex.Order[d.RelPath]; ok {
			sortOrder = order
		}

		group := docsGroupName(d.RelPath)
		groups[group] = append(groups[group], rankedEntry{
			entry: navEntry{
				Title:  title,
				Href:   d.HTMLFile,
				Active: d.HTMLFile == active,
			},
			order: sortOrder,
		})
	}

	out := navSection{
		Title: "Docs",
		Groups: []navGroup{{
			Entries: []navEntry{{
				Title:  "Overview",
				Href:   "index.html",
				Active: active == "index.html",
			}},
		}},
	}

	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Slice(groupNames, func(i, j int) bool {
		if groupNames[i] == "General" {
			return true
		}
		if groupNames[j] == "General" {
			return false
		}
		return strings.ToLower(groupNames[i]) < strings.ToLower(groupNames[j])
	})

	for _, group := range groupNames {
		entries := groups[group]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].order != entries[j].order {
				return entries[i].order < entries[j].order
			}
			return strings.ToLower(entries[i].entry.Title) < strings.ToLower(entries[j].entry.Title)
		})

		groupNav := navGroup{Title: group}
		groupNav.Entries = make([]navEntry, 0, len(entries))
		for _, ranked := range entries {
			groupNav.Entries = append(groupNav.Entries, ranked.entry)
		}
		out.Groups = append(out.Groups, groupNav)
	}

	return out
}

func buildGoSection(title, section string, apiDocs []apiDoc, active string) navSection {
	groups := map[string][]navEntry{}

	for _, d := range apiDocs {
		if d.Section != section {
			continue
		}

		tail := strings.TrimPrefix(d.ShortPath, section+"/")
		if tail == d.ShortPath {
			tail = d.ShortPath
		}
		tail = strings.TrimSpace(tail)
		if tail == "" {
			continue
		}

		group := filepath.ToSlash(filepath.Dir(tail))
		if group == "." {
			group = ""
		}
		linkTitle := filepath.Base(tail)
		if linkTitle == "." || linkTitle == "/" || linkTitle == "" {
			linkTitle = tail
		}

		groups[group] = append(groups[group], navEntry{
			Title:  linkTitle,
			Href:   d.HTMLFile,
			Active: d.HTMLFile == active,
		})
	}

	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Slice(groupNames, func(i, j int) bool {
		if groupNames[i] == "" {
			return true
		}
		if groupNames[j] == "" {
			return false
		}
		return strings.ToLower(groupNames[i]) < strings.ToLower(groupNames[j])
	})

	out := navSection{Title: title}
	for _, group := range groupNames {
		entries := groups[group]
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
		})
		groupTitle := group
		if groupTitle == "" {
			groupTitle = "Core"
		}
		out.Groups = append(out.Groups, navGroup{
			Title:   groupTitle,
			Entries: entries,
		})
	}
	return out
}

func docsGroupName(relPath string) string {
	if !strings.HasPrefix(relPath, "docs/") {
		return "General"
	}
	inside := strings.TrimPrefix(relPath, "docs/")
	dir := filepath.ToSlash(filepath.Dir(inside))
	if dir == "." || dir == "" {
		return "General"
	}
	return dir
}

func resolveMarkdownRelPath(currentRel, target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false
	}
	if strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#") {
		return "", false
	}

	if idx := strings.Index(target, "#"); idx >= 0 {
		target = target[:idx]
	}
	target = strings.TrimPrefix(target, "./")
	target = strings.TrimPrefix(target, "/")

	rel := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(currentRel), target)))
	if rel == "." || rel == "" {
		return "", false
	}
	return rel, true
}

func renderPage(path string, data pageData) error {
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func uniqueHTMLName(base string, used map[string]int) string {
	if used[base] == 0 {
		used[base] = 1
		return base + ".html"
	}
	used[base]++
	return fmt.Sprintf("%s-%d.html", base, used[base]-1)
}

func markdownTitle(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
	}
	return fallback
}

func findProjectDoc(docs []markdownDoc, relPath string) (markdownDoc, bool) {
	for _, d := range docs {
		if d.RelPath == relPath {
			return d, true
		}
	}
	return markdownDoc{}, false
}

func packageNameForDir(dir string) (name string, hasGoFiles bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if strings.HasSuffix(filename, ".go") && !strings.HasSuffix(filename, "_test.go") {
			files = append(files, filepath.Join(dir, filename))
		}
	}
	if len(files) == 0 {
		return "", false, nil
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	for _, path := range files {
		file, parseErr := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if parseErr != nil {
			return "", true, parseErr
		}
		if file != nil && file.Name != nil {
			return file.Name.Name, true, nil
		}
	}
	return "", true, nil
}

func shortenImportPath(modulePath, importPath string) string {
	if modulePath == "" || !strings.HasPrefix(importPath, modulePath) {
		return importPath
	}
	short := strings.TrimPrefix(importPath, modulePath)
	short = strings.TrimPrefix(short, "/")
	if short == "" {
		return importPath
	}
	return short
}

func slugify(input string) string {
	input = strings.ToLower(input)
	var out strings.Builder
	lastDash := false

	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}

	slug := strings.Trim(out.String(), "-")
	if slug == "" {
		return "doc"
	}
	return slug
}

func detectModulePath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if modulePath == "" {
				break
			}
			return modulePath, nil
		}
	}
	return "", fmt.Errorf("%s: module directive not found", goModPath)
}

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDirIfExists(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return nil
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyAppPreviewAssets(root, outDir string) error {
	appsDir := filepath.Join(root, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}

		src := filepath.Join(appsDir, name, "preview.png")
		if _, statErr := os.Stat(src); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return statErr
		}

		dst := filepath.Join(outDir, "assets", "apps", name, "preview.png")
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	return nil
}

func copyOptionalFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return copyFile(src, dst)
}
