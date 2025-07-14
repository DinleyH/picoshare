package handlers

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mtlynch/picoshare/v2/picoshare"
)

func (s Server) playlistsGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/playlists.html")

	type props struct {
		commonProps
		Playlists []picoshare.Playlist
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Use s.store, which is defined in your server.go
		playlists, err := s.store.GetPlaylists()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := t.Execute(w, props{
			commonProps: makeCommonProps("PicoShare - Playlists", r.Context()),
			Playlists:   playlists,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// ...

// playlistEntryDelete handles removing a file entry from a playlist.
func (s Server) playlistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		playlistID, pOK := vars["playlist_id"]
		entryID, eOK := vars["entry_id"]
		if !pOK || !eOK {
			http.Error(w, "Playlist ID and Entry ID are required in URL", http.StatusBadRequest)
			return
		}

		err := s.store.RemoveEntryFromPlaylist(picoshare.PlaylistID(playlistID), picoshare.EntryID(entryID))
		if err != nil {
			log.Printf("failed to remove entry %s from playlist %s: %v", entryID, playlistID, err)
			http.Error(w, "Failed to remove file from playlist", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s Server) playlistEditGet() http.HandlerFunc {

	fns := template.FuncMap{
		"formatFileSize": humanReadableFileSize,
	}

	t := parseTemplatesWithFuncs(fns, "templates/pages/edit-playlists.html")

	type props struct {
		commonProps
		// The template will now receive a single playlist object.
		PlaylistData   picoshare.PlaylistData
		PlaylistFiles  []picoshare.UploadMetadata
		AvailableFiles []picoshare.UploadMetadata
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		playlistId, ok := vars["id"]
		if !ok {
			http.Error(w, "Playlist ID is missing in URL", http.StatusBadRequest)
			return
		}

		playlistResult, err := s.store.GetPlaylistsData(picoshare.PlaylistID(playlistId))
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Playlist not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		playlistFiles, err := s.store.GetPlaylistEntries(picoshare.PlaylistID(playlistId))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a map of IDs for easy lookup
		playlistFileIDs := make(map[picoshare.EntryID]bool)
		for _, pf := range playlistFiles {
			playlistFileIDs[pf.ID] = true
		}

		// Get all files from the server
		allFiles, err := s.store.GetEntriesMetadata()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Filter allFiles to get only the ones NOT in the playlist
		var availableFiles []picoshare.UploadMetadata
		for _, file := range allFiles {
			if !playlistFileIDs[file.ID] {
				availableFiles = append(availableFiles, file)
			}
		}
		// The title can be updated to be more specific.
		p := props{
			commonProps:    makeCommonProps("PicoShare - Edit Playlist", r.Context()),
			PlaylistData:   playlistResult,
			PlaylistFiles:  playlistFiles,
			AvailableFiles: availableFiles,
		}

		if err := t.Execute(w, p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (s Server) playlistsPost() http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}
	type response struct {
		ID           string    `json:"id"`
		Name         string    `json:"name"`
		CreationTime time.Time `json:"creation_time"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Use s.store, which is defined in your server.go
		playlist, err := s.store.CreatePlaylist(req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response{
			ID:           playlist.ID,
			Name:         playlist.Name,
			CreationTime: playlist.CreationTime,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (s Server) playlistPut() http.HandlerFunc {
	type request struct {
		Name string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		playlistId, ok := vars["id"]
		if !ok {
			http.Error(w, "Playlist ID is missing in URL", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "Playlist name cannot be empty", http.StatusBadRequest)
			return
		}

		err := s.store.UpdatePlaylistName(picoshare.PlaylistID(playlistId), req.Name)
		if err != nil {
			http.Error(w, "Failed to update playlist", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s Server) playlistEntryPost() http.HandlerFunc {
	type request struct {
		FileID string `json:"file_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		playlistID, ok := vars["playlist_id"]
		if !ok {
			http.Error(w, "Playlist ID is missing in URL", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		err := s.store.AddEntryToPlaylist(picoshare.PlaylistID(playlistID), picoshare.EntryID(req.FileID))
		if err != nil {
			http.Error(w, "Failed to add file to playlist", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func (s Server) playlistViewGet() http.HandlerFunc {
	// Add the new template to the list of parsed files.
	t := parseTemplates("templates/layouts/base.html", "templates/pages/playlist-view.html")

	// Define the properties required by the template.
	type props struct {
		commonProps
		PlaylistData  picoshare.PlaylistData
		PlaylistFiles []picoshare.UploadMetadata
		CurrentVideo  picoshare.UploadMetadata
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		// The id from the URL will not have the "pl-" prefix
		playlistID, ok := vars["id"]
		if !ok {
			http.Error(w, "Playlist ID is missing in URL", http.StatusBadRequest)
			return
		}

		// Get the playlist metadata (like name, creation date)
		playlistData, err := s.store.GetPlaylistsData(picoshare.PlaylistID(playlistID))
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Playlist not found", http.StatusNotFound)
			} else {
				log.Printf("failed to get playlist data for %s: %v", playlistID, err)
				http.Error(w, "Error retrieving playlist", http.StatusInternalServerError)
			}
			return
		}

		// Get all the files/videos in the playlist
		playlistFiles, err := s.store.GetPlaylistEntries(picoshare.PlaylistID(playlistID))
		if err != nil {
			log.Printf("failed to get playlist entries for %s: %v", playlistID, err)
			http.Error(w, "Error retrieving playlist entries", http.StatusInternalServerError)
			return
		}

		// If the playlist is empty, we can't show anything.
		if len(playlistFiles) == 0 {
			http.Error(w, "This playlist is empty.", http.StatusNotFound)
			return
		}

		// Default to the first video in the playlist.
		currentVideo := playlistFiles[0]
		// Allow overriding via query parameter, e.g., /pl-XXXX?v=YYYY
		if videoID := r.URL.Query().Get("v"); videoID != "" {
			for _, pf := range playlistFiles {
				if string(pf.ID) == videoID {
					currentVideo = pf
					break
				}
			}
		}

		// Execute the template with the data.
		p := props{
			commonProps:   makeCommonProps("PicoShare - "+playlistData.Name, r.Context()),
			PlaylistData:  playlistData,
			PlaylistFiles: playlistFiles,
			CurrentVideo:  currentVideo,
		}

		if err := t.Execute(w, p); err != nil {
			log.Printf("failed to execute template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
