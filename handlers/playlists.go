package handlers

import (
	"encoding/json"
	"net/http"
	"time"

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