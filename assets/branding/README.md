# Engram Brand Assets

Canonical SVG icons for the Engram brand. Single source of truth — copy into
deployable locations via `scripts/sync-branding.ps1`.

## Files

| File | Use |
|------|-----|
| `engram-icon.svg` | Master 1024×1024 — marketing, OG image, app icon source |
| `engram-icon-512.svg` | Optimized 512×512 — large UI, social cards |
| `engram-icon-256.svg` | Optimized 256×256 — README header, marketplace listings, apple-touch-icon |
| `favicon.svg` | 64×64 — generic favicon, header logo |
| `favicon-32.svg` | Optical 32×32 — browser tab, sidebar logo |
| `favicon-16.svg` | Optical 16×16 — small UI chrome |
| `*-transparent.svg` | Mark only (no dark canvas) — for use on already-dark surfaces |

## Color tokens

```yaml
ink:           "#0C0F1A"   # background
orange:        "#EE7410"   # brand mark
orange-soft:   "#FFB272"   # accent text
```

## Locations the assets are copied to

| Path | Purpose |
|------|---------|
| `ui/public/branding/` | Engram dashboard (Vite static) — served at `/branding/` |
| `ui/public/favicon.svg` | Dashboard favicon root path |
| `docs/public/branding/` | Engram docs site — served at `/branding/` |
| `docs/public/favicon.svg` | Docs favicon root path |

Run `scripts/sync-branding.ps1` after editing any asset here to propagate
the change to all deployable locations.

## Live gallery

Open `ui/public/branding/index.html` (or visit `/branding/` on the dashboard
origin) for a rendered swatch sheet with usage examples.

## Don'ts

- Don't recolor the orange mark — `#EE7410` only.
- Don't apply drop shadows, gradients, or strokes.
- Don't use the 1024 master inline at small sizes — pick the closest variant.
- Don't use `favicon.svg` below 48 px — pick `favicon-32.svg` / `favicon-16.svg` for crisp pixels.
- Don't put the transparent mark on a light or orange surface — picks up no contrast.
