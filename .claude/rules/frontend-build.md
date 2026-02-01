---
paths:
  - "cmd/gollem/frontend/src/**/*"
  - "cmd/gollem/frontend/index.html"
  - "cmd/gollem/frontend/tailwind.config.js"
  - "cmd/gollem/frontend/vite.config.ts"
  - "cmd/gollem/frontend/postcss.config.js"
---

After editing frontend source files, run `pnpm --dir cmd/gollem/frontend build` to rebuild the production bundle before finishing the task. The built `dist/` directory is embedded into the Go binary via `go:embed` and must be kept up to date.