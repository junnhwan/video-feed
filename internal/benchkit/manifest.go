package benchkit

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Manifest struct {
	RunID     string          `json:"run_id"`
	BaseURL   string          `json:"base_url"`
	Password  string          `json:"password"`
	UsersCSV  string          `json:"users_csv,omitempty"`
	VideosCSV string          `json:"videos_csv,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	Users     []ManifestUser  `json:"users"`
	Videos    []ManifestVideo `json:"videos"`
	Tags      []string        `json:"tags,omitempty"`
	HotAsOf   int64           `json:"hot_as_of,omitempty"`
}

type ManifestUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type ManifestVideo struct {
	ID         uint   `json:"id"`
	AuthorID   uint   `json:"author_id"`
	Title      string `json:"title"`
	Popularity int64  `json:"popularity"`
	LikesCount int64  `json:"likes_count"`
}

func WriteManifest(path string, manifest Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func ReadManifest(path string) (Manifest, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func WriteUsersCSV(path string, users []ManifestUser) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"account_id", "username"}); err != nil {
		return err
	}
	for _, user := range users {
		if err := writer.Write([]string{strconv.FormatUint(uint64(user.ID), 10), user.Username}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func WriteVideosCSV(path string, videos []ManifestVideo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"video_id", "author_id", "popularity", "likes_count"}); err != nil {
		return err
	}
	for _, item := range videos {
		if err := writer.Write([]string{
			strconv.FormatUint(uint64(item.ID), 10),
			strconv.FormatUint(uint64(item.AuthorID), 10),
			strconv.FormatInt(item.Popularity, 10),
			strconv.FormatInt(item.LikesCount, 10),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
