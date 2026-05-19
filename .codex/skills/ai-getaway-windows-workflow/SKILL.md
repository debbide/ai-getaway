---
name: ai-getaway-windows-workflow
description: Windows-specific workflow notes for the D:\python\ai-getaway repository. Use when Codex works in this project to choose compatible search commands, Go cache settings, frontend build validation, and local verification behavior.
---

# 星空 AI Windows Workflow

Use these project-specific notes before running commands in `D:\python\ai-getaway`.

## Environment

- Treat the shell as Windows PowerShell.
- Follow `AGENTS.md`: do not do local page/browser validation for this repo; build passing is enough.
- The project has a Go backend at repo root and a Vue frontend in `frontend/`.

## Search

- Try `rg` first only if available.
- If `rg` is not installed, use PowerShell:
  - File listing: `Get-ChildItem -Path frontend\src -Recurse -File | Select-Object FullName`
  - Text search: `Get-ChildItem -Path frontend\src,controller,service,router,model -Recurse -File | Select-String -Pattern 'pattern1','pattern2' -SimpleMatch | Select-Object Path,LineNumber,Line`
  - Line-numbered snippets: `$p='path\file.go'; $i=1; Get-Content -Path $p | ForEach-Object { if($i -ge 10 -and $i -le 80){ '{0,5}: {1}' -f $i,$_ }; $i++ }`

## Go Validation

- Run `gofmt -w` on changed Go files.
- The default Go build cache under `C:\Users\63138\AppData\Local\go-build` may fail with `Access is denied`.
- Prefer repo-local caches for tests:

```powershell
$env:GOCACHE='D:\python\ai-getaway\.gocache'; $env:GOMODCACHE='D:\python\ai-getaway\.gomodcache'; go test ./...
```

## Frontend Validation

- For frontend changes, run the production build from `frontend`:

```powershell
npm run build
```

- Do not start Vite or browser-preview pages unless the user explicitly asks. The repo instruction says build passing is sufficient.
- `npm run build` may emit PowerShell `Test-Path ... Access is denied` from `npm.ps1` even when Vite finishes successfully; judge success by the command exit code and the Vite `built` output.

## Editing Notes

- Use `apply_patch` for manual file edits.
- Do not undo unrelated user changes in the working tree.
- Keep API additions aligned with the existing Gin route groups:
  - authenticated user routes under `/api`
  - admin routes under `/api/admin` with `middleware.AdminOnly()`
