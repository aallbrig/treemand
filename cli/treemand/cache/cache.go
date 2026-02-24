// Package cache provides SQLite-backed caching for discovered CLI trees.
package cache

import (
"crypto/sha256"
"database/sql"
"encoding/json"
"fmt"
"os"
"os/exec"
"path/filepath"
"strings"
"time"

_ "github.com/mattn/go-sqlite3" // sqlite3 driver

"github.com/aallbrig/treemand/models"
)

// Cache stores and retrieves discovered CLI trees.
type Cache struct {
db *sql.DB
}

// Open opens (or creates) the cache database at dir/cache.db.
func Open(dir string) (*Cache, error) {
if err := os.MkdirAll(dir, 0o755); err != nil {
return nil, fmt.Errorf("create cache dir: %w", err)
}
dbPath := filepath.Join(dir, "cache.db")
db, err := sql.Open("sqlite3", dbPath)
if err != nil {
return nil, fmt.Errorf("open sqlite3: %w", err)
}
c := &Cache{db: db}
if err := c.migrate(); err != nil {
db.Close()
return nil, err
}
return c, nil
}

// Close closes the underlying database.
func (c *Cache) Close() error { return c.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS trees (
key       TEXT PRIMARY KEY,
cli       TEXT NOT NULL,
version   TEXT NOT NULL,
strategy  TEXT NOT NULL,
data      TEXT NOT NULL,
cached_at INTEGER NOT NULL
);
`

func (c *Cache) migrate() error {
_, err := c.db.Exec(schema)
return err
}

// cacheSchemaVersion is bumped whenever parsing logic changes significantly,
// forcing old cached entries to be ignored.
const cacheSchemaVersion = "v3"

// Key produces a cache key from cli name, version string, and strategies list.
func Key(cli, version string, strategies []string) string {
s := cli + "|" + version + "|" + strings.Join(strategies, ",") + "|" + cacheSchemaVersion
h := sha256.Sum256([]byte(s))
return fmt.Sprintf("%x", h[:8])
}

// Get retrieves a cached tree. Returns nil, nil if not found or expired.
func (c *Cache) Get(key string, maxAge time.Duration) (*models.Node, error) {
row := c.db.QueryRow(`SELECT data, cached_at FROM trees WHERE key = ?`, key)
var data string
var cachedAt int64
if err := row.Scan(&data, &cachedAt); err == sql.ErrNoRows {
return nil, nil
} else if err != nil {
return nil, err
}
if maxAge > 0 && time.Since(time.Unix(cachedAt, 0)) > maxAge {
return nil, nil // expired
}
var node models.Node
if err := json.Unmarshal([]byte(data), &node); err != nil {
return nil, err
}
return &node, nil
}

// Put stores a tree in the cache.
func (c *Cache) Put(key, cli, version, strategy string, node *models.Node) error {
data, err := json.Marshal(node)
if err != nil {
return err
}
_, err = c.db.Exec(
`INSERT OR REPLACE INTO trees (key, cli, version, strategy, data, cached_at) VALUES (?,?,?,?,?,?)`,
key, cli, version, strategy, string(data), time.Now().Unix(),
)
return err
}

// Delete removes an entry from the cache.
func (c *Cache) Delete(key string) error {
_, err := c.db.Exec(`DELETE FROM trees WHERE key = ?`, key)
return err
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
_, err := c.db.Exec(`DELETE FROM trees`)
return err
}

// ClearCLI removes all cached entries for a specific CLI name.
func (c *Cache) ClearCLI(cli string) error {
_, err := c.db.Exec(`DELETE FROM trees WHERE cli = ?`, cli)
return err
}

// ListCLIs returns the names of all CLIs currently in the cache.
func (c *Cache) ListCLIs() ([]string, error) {
rows, err := c.db.Query(`SELECT DISTINCT cli FROM trees ORDER BY cli`)
if err != nil {
return nil, err
}
defer rows.Close()
var names []string
for rows.Next() {
var name string
if err := rows.Scan(&name); err != nil {
return nil, err
}
names = append(names, name)
}
return names, rows.Err()
}


// CLIVersion attempts to get the version string for a CLI by running <cli> --version.
func CLIVersion(cli string) string {
cmd := exec.Command(cli, "--version") //nolint:gosec
out, err := cmd.CombinedOutput()
if err != nil || len(out) == 0 {
return "unknown"
}
line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
if len(line) > 64 {
line = line[:64]
}
return line
}
