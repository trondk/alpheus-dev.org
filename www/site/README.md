# alpheus-dev.org

The website for Alpheus, built with Hugo. The requirements are in
[`../SPEC.md`](../SPEC.md).

```sh
hugo server          # preview on http://localhost:1313/
hugo --gc --minify   # build to public/
```

Hugo is pinned to v0.168.0 (WWW-21).

| What | Where |
|---|---|
| Project phase shown on the front page (WWW-8) | `params.phase` in `hugo.toml` |
| Repository behind the Go vanity import (WWW-19) | `params.repo` in `hugo.toml` |
| Public Go packages (WWW-17) | `data/gopackages.toml`, generated in CI |
| Front page text | `content/_index.md` |
| Text for each phase | `data/phases.toml` |
| Theme: layouts, CSS, fonts, river | `themes/alpheus/` |

Fonts are under the SIL Open Font License; the license texts are next to
them in `themes/alpheus/static/fonts/`.

## CI

`.github/workflows/site.yml` builds the site, checks it and publishes it
to GitHub Pages. It expects to be at the root of `alpheus-dev/alpheus-dev.org`.

| Check | Requirement | Run locally |
|---|---|---|
| Colour contrast of the palette | WWW-9, WWW-41 | `python3 ci/check_contrast.py` |
| No external resources, front page under 150 KB | WWW-25, WWW-26 | `python3 ci/check_site.py` (after `hugo`) |
| axe on the front page, privacy page and `/alpheus/` | WWW-34 | `npx pa11y-ci@4.1.1 --config ci/pa11yci.json` (serve `public/` on port 8080) |
| Lighthouse, at least 95 | WWW-27 | `npx @lhci/cli@0.15.1 autorun --config=ci/lighthouserc.json` |
| Vanity pages answer `?go-get=1` | WWW-17 | after deploy only |
| `go list` straight from the site | WWW-20 | after deploy, once the variable `GO_MODULE_PUBLISHED` is `true` |

Before the first deploy, set *Settings → Pages → Source* to *GitHub Actions*.
