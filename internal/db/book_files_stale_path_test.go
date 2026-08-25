package db

// Regression tests for #2186: BookFileRepo.Add is INSERT OR IGNORE, so
// registering a file at a new location appends a row instead of replacing the
// old one. The derivation used to take the lowest id unconditionally, so a
// book whose folder had been renamed rendered the dead path forever while
// still reporting Imported.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vavallee/bindery/internal/models"
)

// writeFileAt creates path and any missing parents.
func writeFileAt(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("book"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestRefreshBookStatus_MovedFileRendersLivePath is the issue repro: register
// a path, rename the folder on disk, register the new path, and the book must
// render the file that is actually there.
func TestRefreshBookStatus_MovedFileRendersLivePath(t *testing.T) {
	database, _, book := openTestDB(t)
	ctx := context.Background()
	repo := NewBookRepo(database)

	root := t.TempDir()
	oldDir := filepath.Join(root, "Old Folder")
	oldPath := writeFileAt(t, filepath.Join(oldDir, "Neuromancer.epub"))

	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, oldPath); err != nil {
		t.Fatalf("AddBookFile old path: %v", err)
	}

	// The user renames the folder on disk; the scan finds the file at its new
	// location and registers it.
	newDir := filepath.Join(root, "New Folder")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatalf("rename folder: %v", err)
	}
	newPath := filepath.Join(newDir, "Neuromancer.epub")
	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, newPath); err != nil {
		t.Fatalf("AddBookFile new path: %v", err)
	}

	got, err := repo.GetByID(ctx, book.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.EbookFilePath != newPath {
		t.Errorf("EbookFilePath = %q, want the live path %q", got.EbookFilePath, newPath)
	}
	if got.FilePath != newPath {
		t.Errorf("legacy FilePath = %q, want the live path %q", got.FilePath, newPath)
	}
	if _, err := os.Stat(got.EbookFilePath); err != nil {
		t.Errorf("rendered path does not exist on disk: %v", err)
	}
	if got.Status != models.BookStatusImported {
		t.Errorf("Status = %q, want imported", got.Status)
	}

	// The stale row is kept, not deleted: it stays visible on the book's Files
	// tab so the user can deregister it.
	files, err := repo.ListFiles(ctx, book.ID)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("want both rows kept, got %d", len(files))
	}
}

// TestRefreshBookStatus_AllFilesMissingKeepsRenderedPath is the hazard test for
// this fix: an unmounted volume makes every path under it vanish at once. The
// derivation must fall back to the pre-fix answer rather than blanking the
// field, so the book keeps its path, keeps reporting Imported, and keeps every
// row. Nothing here may look like "this book lost its files".
func TestRefreshBookStatus_AllFilesMissingKeepsRenderedPath(t *testing.T) {
	database, _, book := openTestDB(t)
	ctx := context.Background()
	repo := NewBookRepo(database)

	root := t.TempDir()
	mount := filepath.Join(root, "mnt", "nas")
	first := writeFileAt(t, filepath.Join(mount, "Neuromancer.epub"))
	second := writeFileAt(t, filepath.Join(mount, "Neuromancer.mobi"))

	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, first); err != nil {
		t.Fatalf("AddBookFile first: %v", err)
	}
	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, second); err != nil {
		t.Fatalf("AddBookFile second: %v", err)
	}

	// The share drops: every tracked path under it stats as ENOENT at once.
	if err := os.RemoveAll(filepath.Join(root, "mnt")); err != nil {
		t.Fatalf("remove mount: %v", err)
	}

	// Something touches the book and forces a re-derivation. Re-registering a
	// path the book already tracks is the real-world shape of that: the row is
	// a no-op (INSERT OR IGNORE) but the status refresh still runs.
	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, first); err != nil {
		t.Fatalf("re-register first: %v", err)
	}

	got, err := repo.GetByID(ctx, book.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.EbookFilePath != first {
		t.Errorf("EbookFilePath = %q, want the unchanged %q", got.EbookFilePath, first)
	}
	if got.Status != models.BookStatusImported {
		t.Errorf("Status = %q, want imported (a missing mount must not flip the book back to wanted)", got.Status)
	}
	files, err := repo.ListFiles(ctx, book.ID)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 rows kept after the mount vanished, got %d", len(files))
	}
}

// TestRefreshBookStatus_UnprovableAbsenceKeepsRow pins the asymmetry the fix
// relies on: only a definite "does not exist" demotes a row. Here the first
// row's parent is a regular file, so os.Stat returns ENOTDIR — "we cannot read
// this", not "this moved" — and the row must keep winning even though a later
// row resolves cleanly. This is what stops a stalled mount from silently
// re-pointing a library at the wrong rows.
func TestRefreshBookStatus_UnprovableAbsenceKeepsRow(t *testing.T) {
	database, _, book := openTestDB(t)
	ctx := context.Background()
	repo := NewBookRepo(database)

	root := t.TempDir()
	blocker := writeFileAt(t, filepath.Join(root, "not-a-dir"))
	unreadable := filepath.Join(blocker, "Neuromancer.epub")
	if _, err := os.Stat(unreadable); os.IsNotExist(err) {
		t.Skipf("platform reports ENOENT rather than ENOTDIR for %q", unreadable)
	}
	live := writeFileAt(t, filepath.Join(root, "Library", "Neuromancer.epub"))

	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, unreadable); err != nil {
		t.Fatalf("AddBookFile unreadable: %v", err)
	}
	if err := repo.AddBookFile(ctx, book.ID, models.MediaTypeEbook, live); err != nil {
		t.Fatalf("AddBookFile live: %v", err)
	}

	got, err := repo.GetByID(ctx, book.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.EbookFilePath != unreadable {
		t.Errorf("EbookFilePath = %q, want %q kept: an unreadable path is not a vanished one",
			got.EbookFilePath, unreadable)
	}
}

// TestBookColumns_FallsBackToBookFilesRow guards the read-path precedence
// change. Migration 028 backfilled an 'ebook' book_files row from the legacy
// untyped file_path column without filling ebook_file_path, so a book can have
// rows while the column is empty. Reading the column first must still fall
// through to the row for that shape.
func TestBookColumns_FallsBackToBookFilesRow(t *testing.T) {
	database, _, book := openTestDB(t)
	ctx := context.Background()
	repo := NewBookRepo(database)
	files := NewBookFileRepo(database)

	// Add the row directly, bypassing refreshBookStatus, so books.ebook_file_path
	// stays empty exactly as the 028 backfill leaves it.
	if err := files.Add(ctx, book.ID, models.MediaTypeEbook, "/legacy/Neuromancer.epub"); err != nil {
		t.Fatalf("files.Add: %v", err)
	}

	got, err := repo.GetByID(ctx, book.ID)
	if err != nil || got == nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.EbookFilePath != "/legacy/Neuromancer.epub" {
		t.Errorf("EbookFilePath = %q, want the book_files row to fill the empty column",
			got.EbookFilePath)
	}
}
