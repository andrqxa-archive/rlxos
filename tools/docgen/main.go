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
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
	BodyHTML   template.HTML
	HTMLFile   string
}

type apiPackage struct {
	ImportPath string
	Dir        string
}

type navEntry struct {
	Title  string
	Href   string
	Active bool
}

type pageData struct {
	Head        template.HTML
	CurrentPath string
	ProjectDocs []navEntry
	APIDocs     []navEntry
	BodyHTML    template.HTML
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
        <section class="doc-nav-group">
          <h2>Project</h2>
          {{ range .ProjectDocs }}
          <a class="doc-link{{ if .Active }} active{{ end }}" href="{{ .Href }}">{{ .Title }}</a>
          {{ end }}
        </section>
        <section class="doc-nav-group">
          <h2>API</h2>
          {{ range .APIDocs }}
          <a class="doc-link{{ if .Active }} active{{ end }}" href="{{ .Href }}">{{ .Title }}</a>
          {{ end }}
        </section>
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
  padding: clamp(1.2rem, 2vw, 2rem);
  min-height: 80vh;
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

	readmeDoc, hasReadme := findProjectDoc(projectDocs, "README.md")

	markdownMap := map[string]string{}
	for i := range projectDocs {
		if projectDocs[i].RelPath == "README.md" {
			markdownMap[projectDocs[i].RelPath] = "index.html"
			continue
		}
		markdownMap[projectDocs[i].RelPath] = projectDocs[i].HTMLFile
	}

	apiPackages, err := collectAPIPackages(cfg.root, cfg.modulePath)
	if err != nil {
		return err
	}
	apiDocs, err := renderAPIDocs(cfg.modulePath, apiPackages)
	if err != nil {
		return err
	}

	if err := copyFile(resolvePath(cfg.root, cfg.themeStyle), filepath.Join(outDir, "styles.css")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "docgen.css"), []byte(docgenStyles), 0644); err != nil {
		return err
	}
	if err := copyDirIfExists(filepath.Join(cfg.root, "docs", "assets"), filepath.Join(outDir, "assets")); err != nil {
		return err
	}
	if err := copyOptionalFile(filepath.Join(cfg.root, "data", "icons", "logo", "logo.png"), filepath.Join(outDir, "assets", "logo.png")); err != nil {
		return err
	}

	for _, d := range projectDocs {
		if d.RelPath == "README.md" {
			continue
		}
		rewritten := rewriteDocLinks(d.Source, d.RelPath, markdownMap)
		body := renderProjectBody(d.Title, d.RelPath, rewritten)
		head := buildHead(d.Title+" | AvyOS Docs", "Project documentation for "+d.Title)
		page := pageData{
			Head:        head,
			CurrentPath: d.HTMLFile,
			ProjectDocs: navForProject(projectDocs, d.HTMLFile),
			APIDocs:     navForAPI(apiDocs, ""),
			BodyHTML:    body,
		}
		if err := renderPage(filepath.Join(outDir, d.HTMLFile), page); err != nil {
			return err
		}
	}

	for _, d := range apiDocs {
		body := renderAPIBody(d)
		head := buildHead(d.ImportPath+" | AvyOS API", "API documentation for "+d.ImportPath)
		page := pageData{
			Head:        head,
			CurrentPath: d.HTMLFile,
			ProjectDocs: navForProject(projectDocs, ""),
			APIDocs:     navForAPI(apiDocs, d.HTMLFile),
			BodyHTML:    body,
		}
		if err := renderPage(filepath.Join(outDir, d.HTMLFile), page); err != nil {
			return err
		}
	}

	var homeBody template.HTML
	homeTitle := "Project Documentation | AvyOS Docs"
	homeDesc := "Project documentation and API reference for AvyOS"
	if hasReadme {
		rewritten := rewriteDocLinks(readmeDoc.Source, readmeDoc.RelPath, markdownMap)
		homeBody = renderReadmeIndexBody(readmeDoc.RelPath, rewritten)
		homeTitle = "Project Documentation | AvyOS Docs"
		homeDesc = "README and project documentation for AvyOS"
	} else {
		homeBody = renderProjectIndexBody(projectDocs)
	}
	home := pageData{
		Head:        buildHead(homeTitle, homeDesc),
		CurrentPath: "index.html",
		ProjectDocs: navForProject(projectDocs, "index.html"),
		APIDocs:     navForAPI(apiDocs, ""),
		BodyHTML:    homeBody,
	}
	if err := renderPage(filepath.Join(outDir, "index.html"), home); err != nil {
		return err
	}

	fmt.Printf("[*] Project docs: %d\n", len(projectDocs))
	fmt.Printf("[*] API docs: %d\n", len(apiDocs))
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

func collectAPIPackages(root, modulePath string) ([]apiPackage, error) {
	var packages []apiPackage

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
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

		if rel != "." && shouldSkipDir(rel) {
			return filepath.SkipDir
		}

		pkgName, hasGoFiles, err := packageNameForDir(path)
		if err != nil {
			return err
		}
		if !hasGoFiles || pkgName == "main" {
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

func renderAPIDocs(modulePath string, packages []apiPackage) ([]apiDoc, error) {
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
		out = append(out, apiDoc{
			ImportPath: pkg.ImportPath,
			Title:      shortenImportPath(modulePath, pkg.ImportPath),
			BodyHTML:   bodyHTML,
			HTMLFile:   htmlName,
		})
	}
	return out, nil
}

func renderProjectBody(title, relPath, source string) template.HTML {
	markdownHTML := renderMarkdown(source)

	var b strings.Builder
	b.WriteString("<h1>" + template.HTMLEscapeString(title) + "</h1>\n")
	b.WriteString(`<p class="muted mono doc-path">` + template.HTMLEscapeString(relPath) + "</p>\n")
	b.WriteString(`<section class="doc-markdown">`)
	b.WriteString(string(markdownHTML))
	b.WriteString("</section>")
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

	if strings.TrimSpace(pkgDoc.Doc) != "" {
		b.WriteString(`<section class="doc-markdown">`)
		b.WriteString(string(renderMarkdown(pkgDoc.Doc)))
		b.WriteString(`</section>`)
	}

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

func renderHomeBody(projectDocs []markdownDoc, apiDocs []apiDoc) template.HTML {
	var b strings.Builder
	b.WriteString("<h1>AvyOS Documentation</h1>\n")
	b.WriteString(`<p class="muted">Generated docs for project guides and API reference.</p>`)

	b.WriteString("<h2>Project Docs</h2>\n")
	b.WriteString(`<ul class="doc-list">`)
	for _, d := range projectDocs {
		b.WriteString(`<li><a href="` + template.HTMLEscapeString(d.HTMLFile) + `">` + template.HTMLEscapeString(d.Title) + "</a></li>")
	}
	b.WriteString("</ul>")

	b.WriteString("<h2>API Reference</h2>\n")
	b.WriteString(`<ul class="doc-list">`)
	for _, d := range apiDocs {
		b.WriteString(`<li><a href="` + template.HTMLEscapeString(d.HTMLFile) + `">` + template.HTMLEscapeString(d.Title) + "</a></li>")
	}
	b.WriteString("</ul>")

	return template.HTML(b.String())
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
	b.WriteString(`<section class="doc-markdown">`)
	b.WriteString(string(renderMarkdown(source)))
	b.WriteString("</section>")
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

func navForProject(docs []markdownDoc, active string) []navEntry {
	entries := []navEntry{{
		Title:  "Overview",
		Href:   "index.html",
		Active: active == "index.html",
	}}

	for _, d := range docs {
		if d.RelPath == "README.md" {
			continue
		}
		entries = append(entries, navEntry{
			Title:  d.Title,
			Href:   d.HTMLFile,
			Active: d.HTMLFile == active,
		})
	}

	if len(entries) > 1 {
		sort.Slice(entries[1:], func(i, j int) bool {
			return strings.ToLower(entries[1+i].Title) < strings.ToLower(entries[1+j].Title)
		})
	}
	return entries
}

func navForAPI(docs []apiDoc, active string) []navEntry {
	entries := make([]navEntry, 0, len(docs))
	for _, d := range docs {
		entries = append(entries, navEntry{
			Title:  d.Title,
			Href:   d.HTMLFile,
			Active: d.HTMLFile == active,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
	})
	return entries
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

func shouldSkipDir(rel string) bool {
	if rel == "." {
		return false
	}

	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return false
	}
	root := parts[0]

	switch root {
	case ".git", "_cache", "external", "tools":
		return true
	}
	return strings.HasPrefix(root, ".")
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

func copyOptionalFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return copyFile(src, dst)
}
