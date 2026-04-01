package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ conn *sql.DB }

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	conn, err := sql.Open("sqlite", filepath.Join(dataDir, "pulpit.db"))
	if err != nil {
		return nil, err
	}
	conn.Exec("PRAGMA journal_mode=WAL")
	conn.Exec("PRAGMA busy_timeout=5000")
	conn.SetMaxOpenConns(4)
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
CREATE TABLE IF NOT EXISTS posts (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    content TEXT DEFAULT '',
    excerpt TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    published INTEGER DEFAULT 0,
    published_at TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
CREATE INDEX IF NOT EXISTS idx_posts_pub ON posts(published, published_at);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT DEFAULT ''
);
`)
	// Default settings
	db.conn.Exec("INSERT OR IGNORE INTO settings (key,value) VALUES ('blog_title','My Blog')")
	db.conn.Exec("INSERT OR IGNORE INTO settings (key,value) VALUES ('blog_description','A self-hosted blog')")
	db.conn.Exec("INSERT OR IGNORE INTO settings (key,value) VALUES ('author','')")
	return err
}

// --- Posts ---

type Post struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Excerpt     string `json:"excerpt"`
	Tags        string `json:"tags"`
	Published   bool   `json:"published"`
	PublishedAt string `json:"published_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (db *DB) CreatePost(title, slug, content, excerpt, tags string, published bool) (*Post, error) {
	id := "post_" + genID(8)
	now := time.Now().UTC().Format(time.RFC3339)
	if slug == "" {
		slug = slugify(title)
	}
	pub := 0
	pubAt := ""
	if published {
		pub = 1
		pubAt = now
	}
	_, err := db.conn.Exec("INSERT INTO posts (id,slug,title,content,excerpt,tags,published,published_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)",
		id, slug, title, content, excerpt, tags, pub, pubAt, now, now)
	if err != nil {
		return nil, err
	}
	return &Post{ID: id, Slug: slug, Title: title, Content: content, Excerpt: excerpt, Tags: tags,
		Published: published, PublishedAt: pubAt, CreatedAt: now, UpdatedAt: now}, nil
}

func (db *DB) ListPosts(publishedOnly bool) ([]Post, error) {
	var query string
	if publishedOnly {
		query = "SELECT id,slug,title,content,excerpt,tags,published,published_at,created_at,updated_at FROM posts WHERE published=1 ORDER BY published_at DESC"
	} else {
		query = "SELECT id,slug,title,content,excerpt,tags,published,published_at,created_at,updated_at FROM posts ORDER BY created_at DESC"
	}
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Post
	for rows.Next() {
		var p Post
		var pub int
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt, &p.Tags, &pub, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
		p.Published = pub == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) GetPost(id string) (*Post, error) {
	var p Post
	var pub int
	err := db.conn.QueryRow("SELECT id,slug,title,content,excerpt,tags,published,published_at,created_at,updated_at FROM posts WHERE id=?", id).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt, &p.Tags, &pub, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	p.Published = pub == 1
	return &p, err
}

func (db *DB) GetPostBySlug(slug string) (*Post, error) {
	var p Post
	var pub int
	err := db.conn.QueryRow("SELECT id,slug,title,content,excerpt,tags,published,published_at,created_at,updated_at FROM posts WHERE slug=? AND published=1", slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Content, &p.Excerpt, &p.Tags, &pub, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	p.Published = pub == 1
	return &p, err
}

func (db *DB) UpdatePost(id string, title, slug, content, excerpt, tags *string, published *bool) (*Post, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if title != nil {
		db.conn.Exec("UPDATE posts SET title=?, updated_at=? WHERE id=?", *title, now, id)
	}
	if slug != nil {
		db.conn.Exec("UPDATE posts SET slug=?, updated_at=? WHERE id=?", *slug, now, id)
	}
	if content != nil {
		db.conn.Exec("UPDATE posts SET content=?, updated_at=? WHERE id=?", *content, now, id)
	}
	if excerpt != nil {
		db.conn.Exec("UPDATE posts SET excerpt=?, updated_at=? WHERE id=?", *excerpt, now, id)
	}
	if tags != nil {
		db.conn.Exec("UPDATE posts SET tags=?, updated_at=? WHERE id=?", *tags, now, id)
	}
	if published != nil {
		pub := 0
		if *published {
			pub = 1
			var existing string
			db.conn.QueryRow("SELECT published_at FROM posts WHERE id=?", id).Scan(&existing)
			if existing == "" {
				db.conn.Exec("UPDATE posts SET published_at=? WHERE id=?", now, id)
			}
		}
		db.conn.Exec("UPDATE posts SET published=?, updated_at=? WHERE id=?", pub, now, id)
	}
	return db.GetPost(id)
}

func (db *DB) DeletePost(id string) error {
	_, err := db.conn.Exec("DELETE FROM posts WHERE id=?", id)
	return err
}

func (db *DB) TotalPosts() int {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count)
	return count
}

// --- Settings ---

func (db *DB) GetSetting(key string) string {
	var val string
	db.conn.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&val)
	return val
}

func (db *DB) SetSetting(key, value string) {
	db.conn.Exec("INSERT OR REPLACE INTO settings (key,value) VALUES (?,?)", key, value)
}

// --- RSS ---

func (db *DB) RSSItems() string {
	posts, _ := db.ListPosts(true)
	var items strings.Builder
	for _, p := range posts {
		pubDate := p.PublishedAt
		if t, err := time.Parse(time.RFC3339, pubDate); err == nil {
			pubDate = t.Format(time.RFC1123Z)
		}
		items.WriteString("<item>")
		items.WriteString("<title>" + xmlEscape(p.Title) + "</title>")
		items.WriteString("<link>/blog/" + xmlEscape(p.Slug) + "</link>")
		desc := p.Excerpt
		if desc == "" && len(p.Content) > 200 {
			desc = p.Content[:200] + "..."
		} else if desc == "" {
			desc = p.Content
		}
		items.WriteString("<description>" + xmlEscape(desc) + "</description>")
		items.WriteString("<pubDate>" + pubDate + "</pubDate>")
		items.WriteString("<guid>" + p.ID + "</guid>")
		items.WriteString("</item>\n")
	}
	return items.String()
}

// --- Stats ---

func (db *DB) Stats() map[string]any {
	var total, published, drafts int
	db.conn.QueryRow("SELECT COUNT(*) FROM posts").Scan(&total)
	db.conn.QueryRow("SELECT COUNT(*) FROM posts WHERE published=1").Scan(&published)
	db.conn.QueryRow("SELECT COUNT(*) FROM posts WHERE published=0").Scan(&drafts)
	return map[string]any{"posts": total, "published": published, "drafts": drafts}
}

// --- Helpers ---

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else if c == ' ' || c == '-' || c == '_' {
			b.WriteByte('-')
		}
	}
	result := b.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func genID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
