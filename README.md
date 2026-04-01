# Stockyard Pulpit

**Blogging platform.** Write in Markdown, publish instantly, RSS out of the box. Not Ghost (requires Node), not WordPress (requires PHP + MySQL). Own your writing forever. Single binary, no external dependencies.

Part of the [Stockyard](https://stockyard.dev) suite of self-hosted developer tools.

## Quick Start

```bash
curl -sfL https://stockyard.dev/install/pulpit | sh
pulpit
```

Blog at [http://localhost:8860/blog](http://localhost:8860/blog)

## Usage

```bash
# Create a post
curl -X POST http://localhost:8860/api/posts \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello World","content":"This is my first post.","tags":"intro","published":true}'

# Public blog
open http://localhost:8860/blog

# RSS feed
open http://localhost:8860/blog/rss
```

## Free vs Pro

| Feature | Free | Pro ($2.99/mo) |
|---------|------|----------------|
| Posts | 10 | Unlimited |
| Public blog | ✓ | ✓ |
| RSS feed | ✓ | ✓ |
| Auto slugs | ✓ | ✓ |
| Custom domain | — | ✓ |
| Analytics | — | ✓ |

## License

Apache 2.0 — see [LICENSE](LICENSE).
