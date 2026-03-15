package server

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/taedi90/deck/internal/deckignore"
)

type browseEntry struct {
	Name string
	Href string
	Kind string
	Size int64
	Meta string
}

type landingLink struct {
	Title string
	Href  string
	Desc  string
}

func (h *serverHandler) handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	fileLinks := make([]landingLink, 0, 4)
	if h.hasBrowseEntries("workflows") {
		fileLinks = append(fileLinks, landingLink{Title: "workflows", Href: "/browse/workflows/", Desc: "Scenarios and components"})
	}
	if h.hasBrowseEntries("files") {
		fileLinks = append(fileLinks, landingLink{Title: "files", Href: "/browse/files/", Desc: "Prepared files"})
	}
	if h.hasBrowseEntries("packages") {
		fileLinks = append(fileLinks, landingLink{Title: "packages", Href: "/browse/packages/", Desc: "Offline package repos"})
	}
	if h.hasDeckBinary() {
		fileLinks = append(fileLinks, landingLink{Title: "deck", Href: "/deck", Desc: "Current binary"})
	}
	imageLinks := make([]landingLink, 0, 1)
	if h.hasImageEntries() {
		imageLinks = append(imageLinks, landingLink{Title: "images", Href: "/browse/images/", Desc: "Image repos and tags"})
	}
	sections := []string{renderHero("main", "server health: ok")}
	if len(fileLinks) > 0 {
		sections = append(sections, renderLandingSection("Files", fileLinks))
	}
	if len(imageLinks) > 0 {
		sections = append(sections, renderLandingSection("Images", imageLinks))
	}
	sections = append(sections, `<footer class="footer"><a class="footer-link" href="/v2/">registry api</a></footer>`)
	body := renderLayout("deck server", strings.Join(sections, ""))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (h *serverHandler) handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/browse/images" || r.URL.Path == "/browse/images/" || strings.HasPrefix(r.URL.Path, "/browse/images/") {
		h.handleBrowseImages(w, r)
		return
	}
	for _, category := range []string{"files", "packages", "workflows"} {
		prefix := "/browse/" + category
		if r.URL.Path == prefix || r.URL.Path == prefix+"/" || strings.HasPrefix(r.URL.Path, prefix+"/") {
			h.handleBrowseDir(w, r, category)
			return
		}
	}
	http.NotFound(w, r)
}

func (h *serverHandler) handleBrowseDir(w http.ResponseWriter, r *http.Request, category string) {
	relPath := strings.TrimPrefix(r.URL.Path, "/browse/"+category)
	relPath = strings.Trim(strings.TrimPrefix(relPath, "/"), "/")
	entries, title, err := h.listBrowseEntries(category, relPath)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		return
	}
	body := renderBrowsePage(title, entries)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (h *serverHandler) listBrowseEntries(category, relPath string) ([]browseEntry, string, error) {
	ignore, err := deckignore.Load(h.rootAbs)
	if err != nil {
		return nil, "", err
	}
	baseDir := category
	if category == "files" || category == "packages" {
		baseDir = filepath.ToSlash(filepath.Join(serverOutputsDir, category))
	}
	fsPath := filepath.Join(h.rootAbs, filepath.FromSlash(baseDir), filepath.FromSlash(relPath))
	info, err := os.Stat(fsPath)
	if err != nil {
		return nil, "", err
	}
	if !info.IsDir() {
		return nil, "", os.ErrNotExist
	}
	list, err := os.ReadDir(fsPath)
	if err != nil {
		return nil, "", err
	}
	entries := make([]browseEntry, 0, len(list)+1)
	baseHref := "/browse/" + category + "/"
	homeHref := "/"
	if relPath != "" {
		parent := path.Dir(strings.Trim(relPath, "/"))
		if parent == "." {
			parent = ""
		}
		parentHref := baseHref
		if parent != "" {
			parentHref += parent + "/"
		}
		entries = append(entries, browseEntry{Name: "..", Href: parentHref, Kind: "dir"})
	} else {
		entries = append(entries, browseEntry{Name: "..", Href: homeHref, Kind: "dir"})
	}
	for _, entry := range list {
		name := entry.Name()
		childRel := filepath.ToSlash(path.Join(relPath, name))
		ignoreRel := filepath.ToSlash(path.Join(baseDir, childRel))
		if ignore.Matches(ignoreRel, entry.IsDir()) {
			continue
		}
		childInfo, err := entry.Info()
		if err != nil {
			return nil, "", err
		}
		href := baseHref + childRel
		kind := "file"
		if entry.IsDir() {
			href += "/"
			kind = "dir"
		} else {
			href = "/" + category + "/" + childRel
		}
		entries = append(entries, browseEntry{Name: name, Href: href, Kind: kind, Size: childInfo.Size()})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name == ".." {
			return true
		}
		if entries[j].Name == ".." {
			return false
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == "dir"
		}
		return entries[i].Name < entries[j].Name
	})
	title := "/browse/" + category + "/"
	if relPath != "" {
		title += relPath
	}
	return entries, title, nil
}

func (h *serverHandler) handleBrowseImages(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/browse/images")
	rel = strings.Trim(rel, "/")
	body, err := h.renderImageBrowse(rel)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func (h *serverHandler) renderImageBrowse(rel string) (string, error) {
	entries, err := h.scanRegistryCatalog()
	if err != nil {
		return "", err
	}
	if rel == "" {
		repos := map[string]bool{}
		for _, entry := range entries {
			repos[entry.repo] = true
		}
		items := make([]browseEntry, 0, len(repos))
		for repo := range repos {
			items = append(items, browseEntry{Name: repo, Href: "/browse/images/" + repo + "/", Kind: "repo"})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		items = append([]browseEntry{{Name: "..", Href: "/", Kind: "dir"}}, items...)
		return renderBrowsePage("/browse/images/", items), nil
	}
	if repoExists(entries, rel) {
		repo := rel
		tags := map[string]bool{}
		for _, entry := range entries {
			if entry.repo == repo {
				tags[entry.tag] = true
			}
		}
		items := []browseEntry{{Name: "..", Href: "/browse/images/", Kind: "dir"}}
		for tag := range tags {
			items = append(items, browseEntry{Name: tag, Href: "/browse/images/" + repo + "/" + tag + "/", Kind: "tag"})
		}
		sort.Slice(items[1:], func(i, j int) bool { return items[1+i].Name < items[1+j].Name })
		return renderBrowsePage("/browse/images/"+repo+"/", items), nil
	}
	repo, tag := splitRepoTag(entries, rel)
	if repo == "" || tag == "" {
		items := []browseEntry{{Name: "..", Href: "/browse/images/", Kind: "dir"}}
		return renderBrowsePage("/browse/images/"+rel+"/", items), nil
	}
	resolved, resolveErr := h.resolveRegistryImage(repo, tag)
	if resolveErr != nil {
		return "", resolveErr
	}
	items := []browseEntry{{Name: "..", Href: "/browse/images/" + repo + "/", Kind: "dir"}}
	items = append(items,
		browseEntry{Name: "repository", Kind: "meta", Meta: resolved.repo},
		browseEntry{Name: "tag", Kind: "meta", Meta: resolved.tag},
		browseEntry{Name: "digest", Kind: "meta", Meta: resolved.digest.String()},
		browseEntry{Name: "archive", Kind: "meta", Meta: resolved.tarPath},
		browseEntry{Name: "registry manifest", Href: "/v2/" + repo + "/manifests/" + tag, Kind: "link", Meta: "open raw manifest"},
	)
	if resolved.manifest != nil {
		for _, layer := range resolved.manifest.Layers {
			items = append(items, browseEntry{Name: "layer", Kind: "meta", Meta: layer.Digest.String()})
		}
	}
	return renderBrowsePage("/browse/images/"+repo+"/"+tag+"/", items), nil
}

func repoExists(entries []registryCatalogEntry, repo string) bool {
	for _, entry := range entries {
		if entry.repo == repo {
			return true
		}
	}
	return false
}

func splitRepoTag(entries []registryCatalogEntry, rel string) (string, string) {
	bestRepo := ""
	bestTag := ""
	for _, entry := range entries {
		prefix := entry.repo + "/"
		if !strings.HasPrefix(rel, prefix) {
			continue
		}
		tag := strings.TrimPrefix(rel, prefix)
		if tag == "" || strings.Contains(tag, "/") {
			continue
		}
		if len(entry.repo) > len(bestRepo) {
			bestRepo = entry.repo
			bestTag = tag
		}
	}
	return bestRepo, bestTag
}

func renderHero(title string, badge string) string {
	return strings.Join([]string{
		`<section class="hero">`,
		`<div>`,
		`<p class="eyebrow"><a href="/">deck server</a></p>`,
		`<h1>` + html.EscapeString(title) + `</h1>`,
		`</div>`,
		`<div class="status-card"><span class="status-dot"></span>` + html.EscapeString(badge) + `</div>`,
		`</section>`,
	}, "")
}

func renderLandingCards(links []landingLink) string {
	cards := make([]string, 0, len(links))
	for _, link := range links {
		cards = append(cards, strings.Join([]string{
			`<a class="card" href="` + html.EscapeString(link.Href) + `">`,
			`<h2>` + html.EscapeString(link.Title) + `</h2>`,
			`<p>` + html.EscapeString(link.Desc) + `</p>`,
			`<span class="card-link">` + cardActionLabel(link.Title) + `</span>`,
			`</a>`,
		}, ""))
	}
	return `<section class="cards cards-fixed">` + strings.Join(cards, "") + `</section>`
}

func cardActionLabel(title string) string {
	if strings.EqualFold(strings.TrimSpace(title), "deck") {
		return "down"
	}
	return "open"
}

func renderLandingSection(title string, links []landingLink) string {
	return `<section class="section"><div class="section-head"><h2>` + html.EscapeString(title) + `</h2></div>` + renderLandingCards(links) + `</section>`
}

func renderBrowsePage(title string, entries []browseEntry) string {
	rows := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := html.EscapeString(entry.Name)
		if entry.Href != "" {
			name = `<a href="` + html.EscapeString(entry.Href) + `">` + name + `</a>`
		}
		kind := entry.Kind
		if kind == "" {
			kind = "file"
		}
		size := "-"
		if entry.Kind == "file" && entry.Size > 0 {
			size = fmt.Sprintf("%d bytes", entry.Size)
		}
		meta := html.EscapeString(entry.Meta)
		if meta == "" {
			meta = "-"
		}
		rows = append(rows, strings.Join([]string{
			"<tr>",
			`<td class="name">` + name + `</td>`,
			`<td><span class="pill pill-` + html.EscapeString(kind) + `">` + html.EscapeString(kind) + `</span></td>`,
			`<td class="size">` + size + `</td>`,
			`<td class="meta">` + meta + `</td>`,
			"</tr>",
		}, ""))
	}
	body := strings.Join([]string{
		renderHero(title, fmt.Sprintf("%d entries", len(entries))),
		`<div class="panel"><table><thead><tr><th>Name</th><th>Type</th><th>Size</th><th>Details</th></tr></thead><tbody>` + strings.Join(rows, "") + `</tbody></table></div>`,
	}, "")
	return renderLayout(title, body)
}

func renderLayout(title string, body string) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</title><style>
body{margin:0;background:#f6f8fb;color:#16202a;font:14px/1.5 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
.shell{max-width:1120px;margin:0 auto;padding:32px 20px 48px}
.hero{display:flex;justify-content:space-between;gap:24px;align-items:flex-start;margin-bottom:24px}.eyebrow{margin:0 0 8px;color:#57606a;text-transform:uppercase;letter-spacing:.08em;font-size:12px;font-weight:700}.eyebrow a{color:inherit;text-decoration:none}
h1{margin:0;font-size:32px;line-height:1.15}
.status-card{display:inline-flex;align-items:center;gap:10px;padding:2px 0;white-space:nowrap;font-weight:600;color:#57606a}.status-dot{width:10px;height:10px;border-radius:999px;background:#1a7f37;box-shadow:0 0 0 4px rgba(26,127,55,.12)}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:16px;margin-bottom:24px}.cards-fixed{grid-template-columns:repeat(auto-fill,minmax(220px,220px));justify-content:start}.card,.panel{background:#fff;border:1px solid #d0d7de;border-radius:16px;box-shadow:0 1px 2px rgba(22,32,42,.04)}
.section{margin-bottom:28px}.section-head{margin:0 0 12px}.section-head h2{margin:0;font-size:20px}
.card{padding:18px;text-decoration:none;color:inherit}.card h2{margin:0 0 8px;font-size:18px}.card p{margin:0;color:#57606a}.card-link{display:inline-block;margin-top:14px;color:#0969da;font-weight:600}
.footer{padding-top:20px;margin-top:auto;color:#57606a;text-align:center}.footer-link{color:#0969da;text-decoration:none;font-weight:600}.panel-inline{padding:16px 18px}.panel{overflow:hidden}table{width:100%;border-collapse:collapse}th,td{padding:14px 16px;border-bottom:1px solid #d8dee4;text-align:left;vertical-align:top}th{font-size:12px;letter-spacing:.06em;text-transform:uppercase;color:#57606a;background:#f6f8fb}tr:last-child td{border-bottom:0}.name a{text-decoration:none;color:#0969da;font-weight:600}.size,.meta{color:#57606a}
.pill{display:inline-flex;align-items:center;border-radius:999px;padding:4px 10px;font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.04em}.pill-dir,.pill-repo{background:#ddf4ff;color:#0550ae}.pill-file,.pill-link{background:#f6f8fa;color:#57606a}.pill-tag{background:#fff8c5;color:#9a6700}.pill-meta{background:#dafbe1;color:#116329}
@media (max-width:720px){.hero{flex-direction:column}.shell{padding:20px 14px 32px}th:nth-child(3),td:nth-child(3),th:nth-child(4),td:nth-child(4){display:none}}
</style></head><body><div class="shell" style="min-height:100vh;display:flex;flex-direction:column;box-sizing:border-box;">`)
	b.WriteString(body)
	b.WriteString(`</div></body></html>`)
	return b.String()
}

func (h *serverHandler) hasBrowseEntries(category string) bool {
	entries, _, err := h.listBrowseEntries(category, "")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name != ".." {
			return true
		}
	}
	return false
}

func (h *serverHandler) hasDeckBinary() bool {
	info, err := os.Stat(filepath.Join(h.rootAbs, "deck"))
	return err == nil && !info.IsDir()
}

func (h *serverHandler) hasImageEntries() bool {
	entries, err := h.scanRegistryCatalog()
	return err == nil && len(entries) > 0
}
