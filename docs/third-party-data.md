# Third-party data terms

Bindery pulls metadata from several public APIs and stores it in your SQLite
database. This page records the usage terms that constrain what a *deployment*
may do with that data, as distinct from
[dependency licences](../CONTRIBUTING.md#dependency-licences), which constrain
what the *binary* may link.

Nothing here restricts ordinary self-hosted use. It matters if you run Bindery
as part of something you charge for, incorporate, or run ads against.

## Hardcover

Source: <https://hardcover.app/pages/policies> (section "API Rules"), plus the
rate limits in <https://docs.hardcover.app/api/getting-started/>. Reviewed
2026-08-14.

Hardcover asserts no new copyright over the database material and states that
it carries "the same licensing as OpenLibrary". There is **no attribution
requirement**, no retention limit, and no expiry clause, so caching Hardcover
responses in the local database indefinitely is fine. Bindery's list sync runs
every 1 to 168 hours, well inside the documented 60 requests per minute.

The API Rules draw one line that matters, between a *personal project* and a
*professional* one (charged for, incorporated, or ad-supported):

- **Personal projects** may use any data from the API.
- **Professional projects** may use only your own personal data and *facts*
  about books, editions, and series. Other users' reviews, ratings, lists, and
  user-generated content are excluded.

Two consequences for anyone running Bindery professionally:

**Aggregated ratings must be excluded.** `books.average_rating` and
`books.ratings_count` are populated from Hardcover by
`internal/hardcoverlistsyncer` and are aggregates of other users' ratings, not
facts about the book. Strip them from any commercial deployment: don't display
them, don't serve them over the API, and don't re-sync them. Title, author,
series, edition, publisher, narrator, description, and cover are facts and are
fine. The setting that controls the sync cadence is `hardcover.sync_interval`.

**Cover images must be self-hosted.** The API Rules permit hot-linking
Hardcover's images from a personal project but prohibit it for a professional
one: "download those images and host them on your own." Bindery's web UI already
does this by routing every cover through `/api/v1/images`, which fetches once
and caches to `<dataDir>/image-cache/`. The OPDS feed does **not** yet do it and
still emits Hardcover URLs directly to reading apps.

> The Hardcover Terms of Service page is unedited template boilerplate whose
> governing-law, arbitration, and liability clauses are literal blanks, and
> whose §5 read literally would prohibit the API use its own documentation
> describes. The API Rules section of the policies page is the operative
> document.

## OpenLibrary

Public domain catalogue data (CC0). Cover images come from
`covers.openlibrary.org` and are subject to their rate limits; Bindery serves
them through the same `/api/v1/images` cache.

## Audible

`internal/metadata/audible` calls an unpublished Amazon endpoint, and Amazon's
Conditions of Use prohibit unauthorised automated access. This is unresolved and
tracked in [#2015](https://github.com/vavallee/bindery/issues/2015), which
proposes an operator opt-out.
