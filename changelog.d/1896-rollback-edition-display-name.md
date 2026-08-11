### Fixed
- **Calibre rollback preview now names the editions it is about to remove**
  ([#1896](https://github.com/vavallee/bindery/issues/1896)) — the label for an
  edition row was resolved by listing the editions of book id `0` and scanning
  that list for a match. No book has id `0`, so the list was always empty and
  every edition row in the preview came back with a blank name. The preview is
  what you read to decide whether to undo an import, and unnamed rows made it
  hard to tell what was in scope. An edition snapshot already carries the
  edition's own row id, so the lookup now goes straight to it via a new
  `EditionRepo.GetByID`, which reads through the rollback transaction so the
  apply path sees its own in-flight changes. Book, author and series-link rows
  were never affected.
