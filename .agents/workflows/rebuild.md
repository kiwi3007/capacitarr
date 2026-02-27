---
description: How to rebuild and redeploy Capacitarr after making changes
---

## Full Rebuild (frontend + backend)

1. Build the frontend:
```bash
cd /home/ghent/src/workspaces/software/capacitarr/capacitarr/frontend
npm run build
```

// turbo
2. Run the rebuild script (stops server, copies assets, rebuilds Go, restarts):
```bash
bash /home/ghent/src/workspaces/software/capacitarr/capacitarr/rebuild.sh
```

## Backend-only Rebuild (no frontend changes)

// turbo
1. Run the rebuild script:
```bash
bash /home/ghent/src/workspaces/software/capacitarr/capacitarr/rebuild.sh
```

## Notes
- The rebuild script is at `capacitarr/rebuild.sh`
- The Go binary is built to `/tmp/capacitarr`
- The server runs on port 8080
- Database (`capacitarr.db`) is preserved across rebuilds
- To clean stale data, delete specific table rows instead of the whole DB
