/**
 * Sync documentation from ../docs/ and root-level project files into
 * content/docs/ for Nuxt Content.
 *
 * Run from the site/ directory: node scripts/sync-docs.mjs
 *
 * Features:
 * - Auto-discovers all .md files in docs/ (no hardcoded file lists)
 * - Excludes docs/plans/ (internal development documents)
 * - Syncs root-level project files (SECURITY.md, CONTRIBUTING.md, etc.)
 * - Rewrites relative markdown links to absolute Nuxt Content paths
 * - Injects CHANGELOG.md with frontmatter
 */
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, join, relative } from 'node:path'

const ROOT = join(import.meta.dirname, '..')
const DOCS_SRC = join(ROOT, '..', 'docs')
const PROJECT_ROOT = join(ROOT, '..')
const CONTENT_DOCS = join(ROOT, 'content', 'docs')

// Directories within docs/ to exclude from sync.
// docs/plans/ contains internal development documents (per .kilocoderules).
const EXCLUDED_DIRS = new Set(['plans'])

/**
 * Recursively discover all .md files in a directory, excluding specified
 * top-level subdirectories.
 */
function discoverMarkdownFiles(dir) {
  const results = []

  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const fullPath = join(dir, entry.name)

    if (entry.isDirectory()) {
      // Check top-level exclusions relative to DOCS_SRC
      const relDir = relative(DOCS_SRC, fullPath)
      if (EXCLUDED_DIRS.has(relDir)) continue

      results.push(...discoverMarkdownFiles(fullPath))
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      results.push(fullPath)
    }
  }

  return results
}

/**
 * Rewrite relative markdown links within docs/ files to absolute /docs/ paths.
 *
 * Matches: [text](file.md) or [text](file.md#anchor)
 * Converts: (file.md) → (/docs/{subdir}/file)
 *
 * Parent-directory links (e.g., ../architecture.md from api/) resolve
 * naturally via URL path normalization in the browser.
 */
function rewriteDocsLinks(content, contentSubdir) {
  const prefix = contentSubdir ? `/docs/${contentSubdir}` : '/docs'
  return content.replace(
    /\]\(([^)]+?)\.md(#[^)]*?)?\)/g,
    (_match, file, anchor = '') => {
      // Skip absolute URLs and already-absolute paths
      if (file.startsWith('http') || file.startsWith('/')) return _match
      // README → index in any directory
      const name = file.replace(/(?:^|(?<=\/))README$/, 'index')
      return `](${prefix}/${name}${anchor})`
    },
  )
}

/**
 * Rewrite markdown links within root-level project files.
 *
 * Root files reference paths relative to the project root:
 * - (CONTRIBUTORS.md)                      → (/docs/contributors)
 * - (docs/architecture.md)                 → (/docs/architecture)
 * - (docs/security/zap-baseline-20260310.md) → (/docs/security/zap-baseline-20260310)
 */
function rewriteRootLinks(content) {
  // Map of root-level file basenames (without .md) to their site paths
  const ROOT_FILE_MAP = {
    SECURITY: 'security',
    CONTRIBUTING: 'contributing',
    CONTRIBUTORS: 'contributors',
    CHANGELOG: 'changelog',
    README: 'index',
    LICENSE: null, // Not synced — leave link unchanged
  }

  return content.replace(
    /\]\(([^)]+?)\.md(#[^)]*?)?\)/g,
    (_match, file, anchor = '') => {
      if (file.startsWith('http') || file.startsWith('/')) return _match

      let path = file

      // Strip docs/ prefix — root files reference docs/ relative to project root
      if (path.startsWith('docs/')) {
        path = path.slice(5)
      } else {
        // Check if this is a known root file
        const mapped = ROOT_FILE_MAP[path]
        if (mapped === null) return _match // Skip unmapped files (e.g., LICENSE)
        if (mapped !== undefined) path = mapped
      }

      // README → index in any directory
      path = path.replace(/(?:^|(?<=\/))README$/, 'index')

      return `](/docs/${path}${anchor})`
    },
  )
}

/**
 * Sync a single file: read, rewrite links, optionally add frontmatter, write.
 */
function syncFile(src, dest, { rewriter = rewriteDocsLinks, rewriterArg = '', frontmatter = null } = {}) {
  mkdirSync(dirname(dest), { recursive: true })

  let content = readFileSync(src, 'utf-8')
  content = rewriter(content, rewriterArg)

  if (frontmatter) {
    content = `---\n${frontmatter}\n---\n\n${content}`
  }

  writeFileSync(dest, content)
}

// ── Auto-discover and sync docs/ ───────────────────────────────────

const discoveredFiles = discoverMarkdownFiles(DOCS_SRC)
let docsCount = 0

for (const srcPath of discoveredFiles) {
  const relPath = relative(DOCS_SRC, srcPath)
  const dir = dirname(relPath)
  const file = basename(relPath)

  // README.md → index.md in any directory
  const destName = file === 'README.md' ? 'index.md' : file
  const destPath = join(CONTENT_DOCS, dir === '.' ? '' : dir, destName)

  // Content subdir for link rewriting (e.g., 'api' for docs/api/*.md)
  const contentSubdir = dir === '.' ? '' : dir

  syncFile(srcPath, destPath, { rewriter: rewriteDocsLinks, rewriterArg: contentSubdir })
  docsCount++
}

// ── Sync root-level project files ──────────────────────────────────

const rootFiles = [
  { src: 'SECURITY.md', dest: 'security.md', title: 'Security Policy' },
  { src: 'CONTRIBUTING.md', dest: 'contributing.md', title: 'Contributing' },
  { src: 'CONTRIBUTORS.md', dest: 'contributors.md', title: 'Contributors' },
]

let rootCount = 0

for (const { src, dest, title } of rootFiles) {
  const srcPath = join(PROJECT_ROOT, src)
  if (!existsSync(srcPath)) {
    console.warn(`⚠ Root file not found, skipping: ${src}`)
    continue
  }
  syncFile(srcPath, join(CONTENT_DOCS, dest), {
    rewriter: rewriteRootLinks,
    frontmatter: `title: ${title}`,
  })
  rootCount++
}

// ── Inject changelog ───────────────────────────────────────────────

const changelogSrc = join(PROJECT_ROOT, 'CHANGELOG.md')
let changelogSynced = false
if (existsSync(changelogSrc)) {
  const changelogContent = readFileSync(changelogSrc, 'utf-8')
  const changelogMd = `---\ntitle: Changelog\n---\n\n${changelogContent}`
  mkdirSync(CONTENT_DOCS, { recursive: true })
  writeFileSync(join(CONTENT_DOCS, 'changelog.md'), changelogMd)
  changelogSynced = true
}

// Screenshots are managed as lossless WebP files in public/screenshots/.
// They are converted once from ../screenshots/*.png using sharp.
// The sync script does not manage them — see docs/plans/ for the conversion process.

console.log(
  `✓ Docs synced to content/docs/ (${docsCount} files from docs/`
  + `, ${rootCount} root files`
  + `${changelogSynced ? ', changelog' : ''})`,
)
