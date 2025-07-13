package sqlite

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mtlynch/picoshare/v2/picoshare"
	"github.com/rs/xid"
)

const (
	timeFormat = time.RFC3339
	// I think Chrome reads in 32768 chunks, but I haven't checked rigorously.
	defaultChunkSize = uint64(32768 * 10)
)

type (
	Store struct {
		ctx       *sql.DB
		chunkSize uint64
	}

	rowScanner interface {
		Scan(...interface{}) error
	}
)

func New(path string, optimizeForLitestream bool) Store {
	return NewWithChunkSize(path, defaultChunkSize, optimizeForLitestream)
}

// NewWithChunkSize creates a SQLite-based datastore with the user-specified
// chunk size for writing files. Most callers should just use New().
func NewWithChunkSize(path string, chunkSize uint64, optimizeForLitestream bool) Store {
	log.Printf("reading DB from %s", path)
	ctx, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := ctx.Exec(`
		PRAGMA temp_store = FILE;
		PRAGMA journal_mode = WAL;
		PRAGMA foreign_keys = 1;
		`); err != nil {
		log.Fatalf("failed to set pragmas: %v", err)
	}

	if optimizeForLitestream {
		if _, err := ctx.Exec(`
			-- Apply Litestream recommendations: https://litestream.io/tips/
			PRAGMA busy_timeout = 5000;
			PRAGMA synchronous = NORMAL;
			PRAGMA wal_autocheckpoint = 0;
				`); err != nil {
			log.Fatalf("failed to set Litestream compatibility pragmas: %v", err)
		}
	}

	applyMigrations(ctx)

	return Store{
		ctx:       ctx,
		chunkSize: chunkSize,
	}
}

func (s *Store) CreatePlaylist(name string) (picoshare.Playlist, error) {
	playlist := picoshare.Playlist{
		ID:           xid.New().String(),
		Name:         name,
		CreationTime: time.Now(),
	}

	_, err := s.ctx.Exec(
		"INSERT INTO playlists (id, name, creation_time) VALUES (?, ?, ?)",
		playlist.ID,
		playlist.Name,
		playlist.CreationTime.Format(time.RFC3339),
	)
	if err != nil {
		return picoshare.Playlist{}, err
	}

	return playlist, nil
}

func (s *Store) GetPlaylists() ([]picoshare.Playlist, error) {
	rows, err := s.ctx.Query("SELECT id, name, creation_time FROM playlists ORDER BY creation_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []picoshare.Playlist
	for rows.Next() {
		var p picoshare.Playlist
		var creationTime string
		if err := rows.Scan(&p.ID, &p.Name, &creationTime); err != nil {
			return nil, err
		}
		p.CreationTime, err = time.Parse(time.RFC3339, creationTime)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}

	return playlists, nil
}

// GetPlaylistEntries retrieves all file entries for a given playlist, sorted.
func (s *Store) GetPlaylistEntries(playlistID picoshare.PlaylistID) ([]picoshare.UploadMetadata, error) {
	// This query now correctly joins with a subquery on entries_data
	// to calculate the file size for each entry.
	rows, err := s.ctx.Query(`
		SELECT
			e.id,
			e.filename,
			e.content_type,
			e.upload_time,
			e.expiration_time,
			sizes.file_size,
			e.note
		FROM playlist_entries pe
		INNER JOIN entries e ON pe.entry_id = e.id
		INNER JOIN (
			SELECT id, SUM(LENGTH(chunk)) AS file_size
			FROM entries_data
			GROUP BY id
		) sizes ON e.id = sizes.id
		WHERE pe.playlist_id = ?
		ORDER BY pe.sort_order ASC`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// The existing scanEntries function will work correctly with this new query.
	return scanEntries(rows)
}

// scanEntries is a helper function to scan sql.Rows into a slice of UploadMetadata.
// scanEntries is a helper function to scan sql.Rows into a slice of UploadMetadata.
func scanEntries(rows *sql.Rows) ([]picoshare.UploadMetadata, error) {
	var entries []picoshare.UploadMetadata
	for rows.Next() {
		var (
			entry          picoshare.UploadMetadata
			uploadTime     string
			expirationTime string
			note           sql.NullString
			// 1. Scan the size into a simple number first.
			fileSizeRaw uint64
		)

		// 2. Update the Scan() call to use the new variable.
		err := rows.Scan(
			&entry.ID,
			&entry.Filename,
			&entry.ContentType,
			&uploadTime,
			&expirationTime,
			&fileSizeRaw, // Use &fileSizeRaw instead of &entry.Size
			&note,
		)
		if err != nil {
			return nil, err
		}

		// 3. Manually create the FileSize type and assign it.
		entry.Size, err = picoshare.FileSizeFromUint64(fileSizeRaw)
		if err != nil {
			return nil, err
		}

		if note.Valid {
			entry.Note.Value = &note.String
		}

		entry.Uploaded, err = parseDatetime(uploadTime)
		if err != nil {
			return nil, err
		}
		exp, err := parseDatetime(expirationTime)
		if err != nil {
			return nil, err
		}
		entry.Expires = picoshare.ExpirationTime(exp)

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetPlaylistsData retrieves a single playlist by its ID.
func (s *Store) GetPlaylistsData(id picoshare.PlaylistID) (picoshare.PlaylistData, error) {
	row := s.ctx.QueryRow("SELECT id, name, creation_time FROM playlists WHERE id = ?", id)

	var p picoshare.PlaylistData
	var creationTime string
	err := row.Scan(&p.ID, &p.Name, &creationTime)
	if err != nil {
		// The caller can check for sql.ErrNoRows to handle "not found" cases.
		return picoshare.PlaylistData{}, err
	}
	p.CreationTime, err = time.Parse(time.RFC3339, creationTime)
	if err != nil {
		return picoshare.PlaylistData{}, err
	}

	return p, nil
}

func (s *Store) UpdatePlaylistName(id picoshare.PlaylistID, name string) error {
	_, err := s.ctx.Exec("UPDATE playlists SET name = ? WHERE id = ?", name, id)
	return err
}

func (s *Store) AddEntryToPlaylist(playlistID picoshare.PlaylistID, entryID picoshare.EntryID) error {
	// Begin a transaction to ensure atomicity. This prevents race conditions
	// where two adds could get the same sort_order.
	tx, err := s.ctx.Begin()
	if err != nil {
		return err
	}
	// Defer a rollback in case anything goes wrong.
	defer tx.Rollback()

	// First, check if the entry already exists in the playlist to avoid duplicates.
	var exists int
	err = tx.QueryRow("SELECT COUNT(*) FROM playlist_entries WHERE playlist_id = ? AND entry_id = ?", playlistID, entryID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		// The entry is already in the playlist, so we can do nothing and return success.
		return nil
	}

	// Get the current maximum sort_order for this playlist.
	var maxSortOrder sql.NullInt64
	err = tx.QueryRow("SELECT MAX(sort_order) FROM playlist_entries WHERE playlist_id = ?", playlistID).Scan(&maxSortOrder)
	if err != nil {
		return err
	}

	// Calculate the new sort order. If there are no entries, start at 1.
	var newSortOrder int64 = 1
	if maxSortOrder.Valid {
		newSortOrder = maxSortOrder.Int64 + 1
	}

	// Insert the new record with the calculated sort order.
	_, err = tx.Exec(
		"INSERT INTO playlist_entries (playlist_id, entry_id, sort_order) VALUES (?, ?, ?)",
		playlistID,
		entryID,
		newSortOrder,
	)
	if err != nil {
		return err
	}

	// If everything succeeded, commit the transaction.
	return tx.Commit()
}

// RemoveEntryFromPlaylist removes a file from a playlist and updates the sort order
// of the remaining items to fill the gap.
func (s *Store) RemoveEntryFromPlaylist(playlistID picoshare.PlaylistID, entryID picoshare.EntryID) error {
	tx, err := s.ctx.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var deletedSortOrder int
	// First, get the sort order of the item we're about to delete.
	err = tx.QueryRow("SELECT sort_order FROM playlist_entries WHERE playlist_id = ? AND entry_id = ?", playlistID, entryID).Scan(&deletedSortOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			// Not an error, the item just isn't in the playlist.
			return nil
		}
		return err
	}

	// Now, delete the entry.
	_, err = tx.Exec("DELETE FROM playlist_entries WHERE playlist_id = ? AND entry_id = ?", playlistID, entryID)
	if err != nil {
		return err
	}

	// Finally, update the sort order for all subsequent items to close the gap.
	_, err = tx.Exec("UPDATE playlist_entries SET sort_order = sort_order - 1 WHERE playlist_id = ? AND sort_order > ?", playlistID, deletedSortOrder)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func formatExpirationTime(et picoshare.ExpirationTime) string {
	return formatTime(time.Time(et))
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

func formatFileLifetime(lt picoshare.FileLifetime) string {
	return lt.String()
}

func parseDatetime(s string) (time.Time, error) {
	return time.Parse(timeFormat, s)
}

func parseFileLifetime(s string) (picoshare.FileLifetime, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return picoshare.FileLifetime{}, err
	}
	return picoshare.NewFileLifetimeFromDuration(d)
}
