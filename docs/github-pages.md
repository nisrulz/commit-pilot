# GitHub Pages

The project website is hosted via GitHub Pages at:

https://nisrulz.github.io/commit-pilot/

## Setup

The site needs no build step; it is a single `index.html` in the repo root.
Deployment runs through the Pages workflow in `.github/workflows/pages.yml`.

**To enable:**

1. Go to repo **Settings → Pages**
2. Under **Source**, select **GitHub Actions**
3. Save

## Updating

Edit `index.html` (or files under `img/`) and push to `main`. The Pages
workflow deploys the change within a minute. Changes to other files, such as
`docs/`, do not trigger a redeploy.
