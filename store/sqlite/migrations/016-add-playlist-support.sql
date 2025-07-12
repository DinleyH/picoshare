-- store/sqlite/migrations/016-add-playlist-support.sql
CREATE TABLE playlists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    creation_time TEXT NOT NULL
) STRICT;

CREATE TABLE playlist_entries (
    playlist_id TEXT NOT NULL,
    entry_id TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    FOREIGN KEY (playlist_id) REFERENCES playlists (id),
    FOREIGN KEY (entry_id) REFERENCES entries (id),
    PRIMARY KEY (playlist_id, entry_id)
) STRICT;