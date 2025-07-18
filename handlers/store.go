package handlers

import (
	"io"

	"github.com/mtlynch/picoshare/v2/picoshare"
)

type Store interface {
	GetEntriesMetadata() ([]picoshare.UploadMetadata, error)
	ReadEntryFile(picoshare.EntryID) (io.ReadSeeker, error)
	GetEntryMetadata(id picoshare.EntryID) (picoshare.UploadMetadata, error)
	InsertEntry(reader io.Reader, metadata picoshare.UploadMetadata) error
	UpdateEntryMetadata(id picoshare.EntryID, metadata picoshare.UploadMetadata) error
	DeleteEntry(id picoshare.EntryID) error
	GetGuestLink(picoshare.GuestLinkID) (picoshare.GuestLink, error)
	GetGuestLinks() ([]picoshare.GuestLink, error)
	InsertGuestLink(picoshare.GuestLink) error
	DeleteGuestLink(picoshare.GuestLinkID) error
	DisableGuestLink(picoshare.GuestLinkID) error
	EnableGuestLink(picoshare.GuestLinkID) error
	InsertEntryDownload(picoshare.EntryID, picoshare.DownloadRecord) error
	GetEntryDownloads(id picoshare.EntryID) ([]picoshare.DownloadRecord, error)
	ReadSettings() (picoshare.Settings, error)
	UpdateSettings(picoshare.Settings) error
	CreatePlaylist(name string) (picoshare.Playlist, error)
	GetPlaylists() ([]picoshare.Playlist, error)
	GetPlaylistsData(id picoshare.PlaylistID) (picoshare.PlaylistData, error)
	UpdatePlaylistName(id picoshare.PlaylistID, name string) error
	AddEntryToPlaylist(playlistID picoshare.PlaylistID, entryID picoshare.EntryID) error
	GetPlaylistEntries(playlistID picoshare.PlaylistID) ([]picoshare.UploadMetadata, error)
	RemoveEntryFromPlaylist(playlistID picoshare.PlaylistID, entryID picoshare.EntryID) error
}
