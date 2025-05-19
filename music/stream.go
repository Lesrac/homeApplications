package music

import (
	"bytes"
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
	BUFFERSIZE     = 8192
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
	defer file.Close()

	ctn, err := io.ReadAll(file)
	if err != nil {
		log.Println(err)
		http.Error(w, "Invalid", http.StatusBadRequest)
		return
	}
	buffer := make([]byte, BUFFERSIZE)

	tempfile := bytes.NewReader(ctn)
	ticker := time.NewTicker(time.Millisecond * DELAY)
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("Could not create flusher")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	w.Header().Add("Content-Type", "audio/mpeg")
	w.Header().Add("Connection", "keep-alive")
	for range ticker.C {
		_, err := tempfile.Read(buffer)
		if err == io.EOF {
			ticker.Stop()
			break
		}
		if _, err := w.Write(buffer); err != nil {
			log.Printf("%s's connection to the audio stream has been closed\n", r.Host)
			return
		}
		flusher.Flush()
		clear(buffer)
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

	songs := musicModels.Songs{
		Songs: []musicModels.Song{
			{Title: "194 Länder"},
			{Title: "Alles nur geklaut"},
			{Title: "APT"},
			{Title: "Guck mal diese Biene da"},
			{Title: "What About Us"},
			{Title: "Wildberry Lillet"},
		},
	}

	log.Println("successfully fetched song titles")
	json.NewEncoder(w).Encode(songs)
}
