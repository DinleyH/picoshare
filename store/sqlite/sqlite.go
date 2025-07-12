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