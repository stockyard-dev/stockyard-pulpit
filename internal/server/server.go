package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/stockyard-dev/stockyard-pulpit/internal/store"
)

type Server struct {
	db     *store.DB
	mux    *http.ServeMux
	port   int
	limits Limits
}

func New(db *store.DB, port int, limits Limits) *Server {
	s := &Server{db: db, mux: http.NewServeMux(), port: port, limits: limits}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/posts", s.handleCreatePost)
	s.mux.HandleFunc("GET /api/posts", s.handleListPosts)
	s.mux.HandleFunc("GET /api/posts/{id}", s.handleGetPost)
	s.mux.HandleFunc("PUT /api/posts/{id}", s.handleUpdatePost)
	s.mux.HandleFunc("DELETE /api/posts/{id}", s.handleDeletePost)

	// Public blog
	s.mux.HandleFunc("GET /blog", s.handleBlogIndex)
	s.mux.HandleFunc("GET /blog/{slug}", s.handleBlogPost)
	s.mux.HandleFunc("GET /blog/rss", s.handleRSS)

	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ui", s.handleUI)
	s.mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"product": "stockyard-pulpit", "version": "0.1.0"})
	})
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[pulpit] listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// --- Post CRUD ---

func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string `json:"title"`
		Slug      string `json:"slug"`
		Content   string `json:"content"`
		Excerpt   string `json:"excerpt"`
		Tags      string `json:"tags"`
		Published bool   `json:"published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		writeJSON(w, 400, map[string]string{"error": "title is required"})
		return
	}
	if s.limits.MaxPosts > 0 {
		total := s.db.TotalPosts()
		if LimitReached(s.limits.MaxPosts, total) {
			writeJSON(w, 402, map[string]string{"error": fmt.Sprintf("free tier limit: %d posts — upgrade to Pro", s.limits.MaxPosts), "upgrade": "https://stockyard.dev/pulpit/"})
			return
		}
	}
	p, err := s.db.CreatePost(req.Title, req.Slug, req.Content, req.Excerpt, req.Tags, req.Published)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"post": p, "url": fmt.Sprintf("/blog/%s", p.Slug)})
}

func (s *Server) handleListPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := s.db.ListPosts(false)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if posts == nil {
		posts = []store.Post{}
	}
	writeJSON(w, 200, map[string]any{"posts": posts, "count": len(posts)})
}

func (s *Server) handleGetPost(w http.ResponseWriter, r *http.Request) {
	p, err := s.db.GetPost(r.PathValue("id"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "post not found"})
		return
	}
	writeJSON(w, 200, map[string]any{"post": p})
}

func (s *Server) handleUpdatePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.db.GetPost(id); err != nil {
		writeJSON(w, 404, map[string]string{"error": "post not found"})
		return
	}
	var req struct {
		Title     *string `json:"title"`
		Slug      *string `json:"slug"`
		Content   *string `json:"content"`
		Excerpt   *string `json:"excerpt"`
		Tags      *string `json:"tags"`
		Published *bool   `json:"published"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	p, err := s.db.UpdatePost(id, req.Title, req.Slug, req.Content, req.Excerpt, req.Tags, req.Published)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"post": p})
}

func (s *Server) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	s.db.DeletePost(r.PathValue("id"))
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// --- Public blog ---

func (s *Server) handleBlogIndex(w http.ResponseWriter, r *http.Request) {
	posts, _ := s.db.ListPosts(true)
	blogTitle := s.db.GetSetting("blog_title")
	blogDesc := s.db.GetSetting("blog_description")

	var content strings.Builder
	for _, p := range posts {
		date := ""
		if p.PublishedAt != "" && len(p.PublishedAt) >= 10 {
			date = p.PublishedAt[:10]
		}
		excerpt := p.Excerpt
		if excerpt == "" && len(p.Content) > 300 {
			excerpt = p.Content[:300] + "..."
		} else if excerpt == "" {
			excerpt = p.Content
		}
		content.WriteString(`<article style="margin-bottom:2.5rem;padding-bottom:2rem;border-bottom:1px solid #2e261e">`)
		content.WriteString(`<a href="/blog/` + he(p.Slug) + `" style="text-decoration:none"><h2 style="font-family:'Libre Baskerville',serif;font-size:1.3rem;color:#f0e6d3;margin:0 0 .3rem">` + he(p.Title) + `</h2></a>`)
		content.WriteString(`<div style="font-family:'JetBrains Mono',monospace;font-size:.65rem;color:#7a7060;margin-bottom:.8rem">` + date)
		if p.Tags != "" {
			for _, tag := range strings.Split(p.Tags, ",") {
				content.WriteString(` · <span style="color:#a0845c">` + he(strings.TrimSpace(tag)) + `</span>`)
			}
		}
		content.WriteString(`</div>`)
		content.WriteString(`<p style="color:#bfb5a3;line-height:1.7">` + he(excerpt) + `</p>`)
		content.WriteString(`<a href="/blog/` + he(p.Slug) + `" style="font-family:'JetBrains Mono',monospace;font-size:.72rem;color:#e8753a">Read more →</a>`)
		content.WriteString(`</article>`)
	}
	if len(posts) == 0 {
		content.WriteString(`<p style="color:#7a7060;text-align:center;padding:3rem;font-style:italic">No posts yet.</p>`)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, blogIndexTemplate, he(blogTitle), he(blogDesc), content.String())
}

func (s *Server) handleBlogPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "rss" {
		s.handleRSS(w, r)
		return
	}
	p, err := s.db.GetPostBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	blogTitle := s.db.GetSetting("blog_title")
	date := ""
	if p.PublishedAt != "" && len(p.PublishedAt) >= 10 {
		date = p.PublishedAt[:10]
	}
	tagsHTML := ""
	if p.Tags != "" {
		for _, tag := range strings.Split(p.Tags, ",") {
			tagsHTML += ` · <span style="color:#a0845c">` + he(strings.TrimSpace(tag)) + `</span>`
		}
	}

	// Simple markdown-ish rendering: paragraphs on double newline
	contentHTML := ""
	for _, para := range strings.Split(p.Content, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if strings.HasPrefix(para, "# ") {
			contentHTML += `<h2 style="margin:1.5rem 0 .8rem;color:#f0e6d3">` + he(para[2:]) + `</h2>`
		} else if strings.HasPrefix(para, "## ") {
			contentHTML += `<h3 style="margin:1.2rem 0 .6rem;color:#f0e6d3">` + he(para[3:]) + `</h3>`
		} else if strings.HasPrefix(para, "```") {
			contentHTML += `<pre style="background:#241e18;padding:1rem;font-family:'JetBrains Mono',monospace;font-size:.78rem;color:#bfb5a3;overflow-x:auto;margin:1rem 0">` + he(strings.TrimPrefix(strings.TrimSuffix(para, "```"), "```")) + `</pre>`
		} else {
			contentHTML += `<p style="margin-bottom:1rem;line-height:1.8;color:#bfb5a3">` + he(para) + `</p>`
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, blogPostTemplate, he(p.Title)+" — "+he(blogTitle), he(p.Title), date, tagsHTML, contentHTML)
}

func (s *Server) handleRSS(w http.ResponseWriter, r *http.Request) {
	blogTitle := s.db.GetSetting("blog_title")
	blogDesc := s.db.GetSetting("blog_description")
	items := s.db.RSSItems()
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>%s</title>
<description>%s</description>
%s
</channel>
</rss>`, he(blogTitle), he(blogDesc), items)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.db.Stats())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func he(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

const blogIndexTemplate = `<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<link rel="alternate" type="application/rss+xml" title="RSS" href="/blog/rss">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>body{background:#1a1410;color:#f0e6d3;font-family:'Libre Baskerville',Georgia,serif;margin:0;min-height:100vh}
.container{max-width:700px;margin:0 auto;padding:2rem 1.5rem}
.header{margin-bottom:2.5rem;padding-bottom:1.5rem;border-bottom:2px solid #8b3d1a}
.header h1{font-size:1.4rem;color:#f0e6d3;margin:0 0 .3rem}
.header p{font-size:.85rem;color:#7a7060}
a{color:#e8753a;text-decoration:none}a:hover{color:#d4a843}
.footer{text-align:center;margin-top:2rem;font-size:.6rem;color:#7a7060}
.footer a{color:#e8753a}
</style></head><body>
<div class="container">
<div class="header"><h1>%s</h1></div>
%s
<div class="footer">Powered by <a href="https://stockyard.dev/pulpit/">Stockyard Pulpit</a> · <a href="/blog/rss">RSS</a></div>
</div></body></html>`

const blogPostTemplate = `<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>body{background:#1a1410;color:#f0e6d3;font-family:'Libre Baskerville',Georgia,serif;margin:0;min-height:100vh}
.container{max-width:700px;margin:0 auto;padding:2rem 1.5rem}
a{color:#e8753a;text-decoration:none}a:hover{color:#d4a843}
.back{font-family:'JetBrains Mono',monospace;font-size:.72rem;color:#a0845c;margin-bottom:2rem;display:block}
h1{font-size:1.6rem;margin:0 0 .5rem;color:#f0e6d3}
.meta{font-family:'JetBrains Mono',monospace;font-size:.65rem;color:#7a7060;margin-bottom:2rem}
.footer{text-align:center;margin-top:3rem;padding-top:1.5rem;border-top:1px solid #2e261e;font-size:.6rem;color:#7a7060}
.footer a{color:#e8753a}
</style></head><body>
<div class="container">
<a href="/blog" class="back">← Back</a>
<h1>%s</h1>
<div class="meta">%s%s</div>
%s
<div class="footer">Powered by <a href="https://stockyard.dev/pulpit/">Stockyard Pulpit</a></div>
</div></body></html>`
