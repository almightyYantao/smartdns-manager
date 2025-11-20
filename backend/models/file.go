package models

import (
	"time"
)

type File struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	FileName  string    `json:"file_name" gorm:"type:varchar(255);not null"`
	FileSize  int64     `json:"file_size"`
	FileType  string    `json:"file_type" gorm:"type:varchar(100)"`
	S3Key     string    `json:"s3_key" gorm:"type:varchar(500);uniqueIndex"`
	S3Bucket  string    `json:"s3_bucket" gorm:"type:varchar(255)"`
	URL       string    `json:"url" gorm:"type:varchar(1000)"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:'active'"` // active, deleted
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UploadRequest struct {
	File     []byte `json:"file" binding:"required"`
	FileName string `json:"file_name" binding:"required"`
	FileType string `json:"file_type"`
	Folder   string `json:"folder"`
}

type UploadResponse struct {
	FileID   uint   `json:"file_id"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
	S3Key    string `json:"s3_key"`
}

type DownloadResponse struct {
	URL      string `json:"url"`
	FileName string `json:"file_name"`
}
