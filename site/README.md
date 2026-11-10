# AWF documentation site

Use Hugo **0.167.0**, Node.js **24.13 or newer**, and npm. GitHub Pages and
Netlify use the same Hugo version. The npm lockfile pins the theme and build
tools; a separate Dart Sass installation is not required.

From this directory:

```sh
npm ci
npm test
npm run build
npm run dev
```

The local site is served at <http://localhost:1313/cli/>. Hugo watches the content,
including the documentation mounted from `../docs`. The development wrapper also
watches `assets/scss` and recompiles styles when they change.

`npm run build` creates the production site in `public/`. Both build and dev
commands fail on Hugo warnings, including deprecations. Additional Hugo arguments
can be passed after `--`, for example `npm run dev -- --port 1314`.

## Stylesheet compatibility

Doks and Bootstrap still use Sass imports and shared global variables.
`scripts/prepare-styles.mjs` resolves the imports in their original order, applies
the official Sass migrator, and compiles them with Dart Sass. It preserves the
site's variable and style overrides without editing dependencies in `node_modules`.
Generated sources and CSS live in the ignored `.generated/` directory.

Every active Sass deprecation is fatal. `npm test` verifies that migrated CSS is
identical to CSS compiled from the original styles. Reassess this adapter when
upstream migrates its styles to modern Sass.

The local stylesheet partial uses Hugo's `templates.Defer` so production PostCSS
and PurgeCSS run after `hugo_stats.json` is complete. Fingerprinting runs last,
ensuring the stylesheet integrity hash matches the published bytes.

## Page descriptions

Explicit descriptions must contain 110–160 characters. Pages without explicit
metadata use a bounded summary, falling back to the page title and site
description when the summary is too short. The site overrides the theme's
description partial to avoid placing whole documentation tables or code examples
in metadata. Keep this override in mind when upgrading the SEO integration.
