package music

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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
	BUFFERSIZE = 16384
	// DELAY should optimally be: track_duration * buffer_size / mp3_file_size
	DELAY          = 150
	MUSIC_DIR      = "music/"
	FILE_EXTENSION = ".mp3"
)

// StreamMusic Idea and implementation proudly taken from https://github.com/Icelain/radio/blob/main/main.go
// Currently not secured as the client uses flutter audioplayers and that one doesn't support headers when calling an
// endpoint
func StreamMusic(w http.ResponseWriter, r *http.Request) {
	middleware.EnableCors(&w)

	filename := strings.TrimPrefix(r.URL.Path, "/audio/")
	if filename == "" {
		log.Println("No file name provided")
		http.Error(w, "No file specified", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName != filename {
		log.Println("File name is not valid")
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}
	nameWithExtension := cleanName + FILE_EXTENSION
	fpath := filepath.Join(MUSIC_DIR, nameWithExtension)

	requestID := uuid.New().String()
	log.Println(requestID, "Streaming file:", fpath)

	if _, err := os.Stat(fpath); err != nil {
		log.Println(requestID, "File does not exist:", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(fpath)
	if err != nil {
		log.Println(requestID, "Error opening file", err)
		http.Error(w, "Invalid", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Println(requestID, "Error closing file:", err)
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		log.Println(requestID, "Error getting file info:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println(requestID, "Could not create flusher")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	w.Header().Add("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Add("Connection", "keep-alive")

	buffer := make([]byte, BUFFERSIZE)
	ticker := time.NewTicker(time.Millisecond * DELAY)
	defer ticker.Stop()
	for range ticker.C {
		n, err := file.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				log.Printf("%s, %s's connection to the audio stream has been closed\n", requestID, r.Host)
				return
			}
			flusher.Flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(requestID, "Error reading file:", err)
			break
		}
	}
	log.Println(requestID, "finished streaming")
}

func FetchSongTitles(w http.ResponseWriter, r *http.Request) {
	log.Println("fetch song titles")
	middleware.EnableCors(&w)
	_, err := middleware.AuthenticateUser(r)
	if err != nil {
		log.Println("auth error: " + err.Error() + " in FetchSongTitles")
		middleware.HandleError(w, err)
		return
	}

	entries, err := os.ReadDir(MUSIC_DIR)
	if err != nil {
		log.Println("read dir error: " + err.Error() + " in FetchSongTitles")
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
		log.Println("json encoder error: " + err.Error() + " in FetchSongTitles")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
