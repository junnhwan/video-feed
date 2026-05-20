package benchkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.json")
	want := Manifest{
		RunID:     "bench-20260520",
		BaseURL:   "http://127.0.0.1:8080",
		Password:  "benchpass",
		UsersCSV:  "users.csv",
		VideosCSV: "videos.csv",
		Users: []ManifestUser{
			{ID: 1, Username: "bench_user_001"},
		},
		Videos: []ManifestVideo{
			{ID: 100, AuthorID: 1, Title: "video 100", Popularity: 42, LikesCount: 7},
		},
	}

	if err := WriteManifest(path, want); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest not written: %v", err)
	}

	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got.RunID != want.RunID || got.UsersCSV != want.UsersCSV || got.Users[0].Username != want.Users[0].Username || got.Videos[0].ID != want.Videos[0].ID {
		t.Fatalf("manifest mismatch: %#v", got)
	}
}

func TestWriteJMeterCSVs(t *testing.T) {
	dir := t.TempDir()
	usersPath := filepath.Join(dir, "users.csv")
	videosPath := filepath.Join(dir, "videos.csv")
	manifest := Manifest{
		Users: []ManifestUser{
			{ID: 1, Username: "u1"},
			{ID: 2, Username: "u2"},
		},
		Videos: []ManifestVideo{
			{ID: 10, AuthorID: 1, Popularity: 100, LikesCount: 3},
		},
	}

	if err := WriteUsersCSV(usersPath, manifest.Users); err != nil {
		t.Fatalf("WriteUsersCSV: %v", err)
	}
	if err := WriteVideosCSV(videosPath, manifest.Videos); err != nil {
		t.Fatalf("WriteVideosCSV: %v", err)
	}

	usersPayload, err := os.ReadFile(usersPath)
	if err != nil {
		t.Fatalf("read users csv: %v", err)
	}
	if got := strings.TrimSpace(string(usersPayload)); got != "account_id,username\n1,u1\n2,u2" {
		t.Fatalf("users csv mismatch:\n%s", got)
	}

	videosPayload, err := os.ReadFile(videosPath)
	if err != nil {
		t.Fatalf("read videos csv: %v", err)
	}
	if got := strings.TrimSpace(string(videosPayload)); got != "video_id,author_id,popularity,likes_count\n10,1,100,3" {
		t.Fatalf("videos csv mismatch:\n%s", got)
	}
}
