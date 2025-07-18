package picoshare

import (
	"io"
	"time"
)

type (
	EntryID        string
	PlaylistID     string
	PlaylistName   string
	Filename       string
	ContentType    string
	ExpirationTime time.Time

	FileNote struct {
		Value *string
	}

	UploadMetadata struct {
		ID            EntryID
		Filename      Filename
		Note          FileNote
		ContentType   ContentType
		Uploaded      time.Time
		Expires       ExpirationTime
		Size          FileSize
		GuestLink     GuestLink
		DownloadCount uint64
	}

	DownloadRecord struct {
		Time      time.Time
		ClientIP  string
		UserAgent string
	}

	UploadEntry struct {
		UploadMetadata
		Reader io.ReadSeeker
	}
)

type Playlist struct {
	ID           string
	Name         string
	CreationTime time.Time
}

type PlaylistData struct {
	ID           string
	Name         string
	CreationTime time.Time
}

// Treat a distant expiration time as sort of a sentinel value signifying a "never expire" option.
var NeverExpire = ExpirationTime(time.Date(2999, time.December, 31, 0, 0, 0, 0, time.UTC))

func (id EntryID) String() string {
	return string(id)
}

func (f Filename) String() string {
	return string(f)
}

func (ct ContentType) String() string {
	return string(ct)
}

func (et ExpirationTime) String() string {
	return et.Time().String()
}

func (et ExpirationTime) Time() time.Time {
	return time.Time(et)
}

func (n FileNote) String() string {
	if n.Value == nil {
		return "<nil>"
	}
	return *n.Value
}
