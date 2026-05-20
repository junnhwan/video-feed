package social

type Social struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	FollowerID uint `gorm:"not null;index:idx_social_follower;uniqueIndex:idx_social_follower_vlogger" json:"follower_id"`
	VloggerID  uint `gorm:"not null;index:idx_social_vlogger;uniqueIndex:idx_social_follower_vlogger" json:"vlogger_id"`
}
