package video

import "regexp"

type Tag struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex;type:varchar(100);not null" json:"name"`
}

type VideoTag struct {
	ID      uint `gorm:"primaryKey" json:"id"`
	VideoID uint `gorm:"index;not null;uniqueIndex:idx_video_tag" json:"video_id"`
	TagID   uint `gorm:"index;not null;uniqueIndex:idx_video_tag" json:"tag_id"`
}

var tagRegex = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)

func ExtractTags(text string) []string {
	matches := tagRegex.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	tags := make([]string, 0, len(matches))
	for _, match := range matches {
		tag := match[1]
		if seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}
