package models

import (
	"time"
)

type Backup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	NodeID      uint      `gorm:"not null;index" json:"node_id"`
	Path        string    `gorm:"type:varchar(500);not null" json:"path"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Size        int64     `json:"size"`
	IsAuto      bool      `gorm:"default:false" json:"is_auto"`
	Comment     string    `gorm:"type:text" json:"comment"`
	Tags        string    `gorm:"type:varchar(255)" json:"tags"`
	IsDeleted   bool      `gorm:"default:false" json:"is_deleted"`
	StorageType string    `gorm:"type:varchar(20);default:'local'" json:"storage_type"` // local 或 s3
	S3Bucket    string    `gorm:"type:varchar(255)" json:"s3_bucket,omitempty"`
	S3Key       string    `gorm:"type:varchar(500)" json:"s3_key,omitempty"`
	S3Region    string    `gorm:"type:varchar(50)" json:"s3_region,omitempty"`
	DownloadURL string    `gorm:"type:varchar(1000)" json:"download_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Node Node `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}
