# Flywall Documentation Site

This directory contains the Hugo-based documentation for Flywall, using the [Docsy](https://www.docsy.dev/) theme.

## Prerequisites

- [Hugo Extended](https://gohugo.io/installation/) 0.110.0 or later
- Go 1.21 or later (for Hugo modules)

## Local Development

```bash
# Install dependencies and start development server
cd docs-site
hugo mod get
hugo server -D
```

The site will be available at http://localhost:1313/

## Building

```bash
hugo --gc --minify
```

Output goes to `public/`.

## Structure

```
docs-site/
├── hugo.toml           # Hugo configuration
├── content/
│   ├── _index.md       # Homepage
│   └── docs/           # Documentation content
│       ├── getting-started/
│       ├── guides/
│       ├── configuration/
│       │   └── reference/  # ← Auto-generated (40+ pages)
│       ├── reference/
│       └── troubleshooting/
└── static/             # Static assets
```

## Configuration Reference

The `content/docs/configuration/reference/` directory contains **auto-generated** documentation from the Go source code. **Do not edit these files manually.**

To regenerate:

```bash
# From project root
./flywall.sh docs hugo

# Or directly
go run ./cmd/gen-config-docs -format=hugo
```

This parses HCL configuration structs in `internal/config/` and generates one Hugo page per top-level block (interface, zone, policy, etc.).

## Adding Content

### New Documentation Page

```bash
hugo new docs/guides/my-new-guide.md
```

### Page Front Matter

```yaml
---
title: "Page Title"
linkTitle: "Short Title"  # For navigation
weight: 10               # Sort order (lower = first)
description: >
  Brief description for SEO and previews.
---
```

## Customization

### Theme Configuration

Edit `hugo.toml` to customize:
- Site title and description
- Navigation menu
- Footer links
- Search settings

### Docsy Features

The Docsy theme provides:
- Responsive design
- Offline search (Lunr.js)
- Diagrams (Mermaid)
- Code highlighting
- Versioned documentation
- Multi-language support

## Deployment

The site is automatically deployed to GitHub Pages when changes are pushed to `main`. See `.github/workflows/docs.yml`.

For custom domain:
1. Add CNAME to `static/CNAME`
2. Configure DNS
3. Update `baseURL` in `hugo.toml`
