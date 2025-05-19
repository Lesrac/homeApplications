package music

import (
	"encoding/json"
	"homeApplications/middleware"
	musicModels "homeApplications/music/models"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BUFFERSIZE = 8192
	// DELAY should optimally be: track_duration * buffer_size / mp3_file_size
	DELAY          = 150
	MUSIC_DIR      = "music/"
	FILE_EXTENSION = ".mp3"
)

// StreamMusic Idea and implementation proudly taken from https://github.com/Icelain/radio/blob/main/main.go
func StreamMusic(w http.ResponseWriter, r *http.Request) {
	middleware.EnableCors(&w)

	filename := strings.TrimPrefix(r.URL.Path, "/audio/")
	if filename == "" {
		http.Error(w, "No file specified", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName != filename {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}
	nameWithExtension := cleanName + FILE_EXTENSION
	fpath := filepath.Join(MUSIC_DIR, nameWithExtension)

	if _, err := os.Stat(fpath); err != nil {
		log.Println("File does not exist:", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(fpath)
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Println("Error closing file:", err)
		}
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("Could not create flusher")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	w.Header().Add("Content-Type", "audio/mpeg")
	w.Header().Add("Connection", "keep-alive")

	buffer := make([]byte, BUFFERSIZE)
	ticker := time.NewTicker(time.Millisecond * DELAY)
	defer ticker.Stop()
	for range ticker.C {
		n, err := file.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				log.Printf("%s's connection to the audio stream has been closed\n", r.Host)
				return
			}
			flusher.Flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Error reading file:", err)
			break
		}
	}

}

func FetchSongTitles(w http.ResponseWriter, r *http.Request) {
	log.Println("fetch song titles")
	middleware.EnableCors(&w)
	_, err := middleware.AuthenticateUser(r)
	if err != nil {
		middleware.HandleError(w, err)
		return
	}

	entries, err := os.ReadDir(MUSIC_DIR)
	if err != nil {
		log.Println("error: " + err.Error() + " in FetchSongTitles")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var directorySongs []musicModels.Song
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), FILE_EXTENSION) {
			entryNameWithoutSuffix := strings.TrimSuffix(entry.Name(), FILE_EXTENSION)
			directorySongs = append(directorySongs, musicModels.Song{Title: entryNameWithoutSuffix})
		}
	}

	songs := musicModels.Songs{
		Songs: directorySongs,
	}

	log.Println("successfully fetched song titles")
	err = json.NewEncoder(w).Encode(songs)
	if err != nil {
		log.Println("error: " + err.Error() + " in encoding JSON")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
