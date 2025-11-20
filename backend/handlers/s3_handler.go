package handlers

import (
	"net/http"
	"strconv"
	"time"

	"smartdns-manager/models"
	"smartdns-manager/services"

	"github.com/gin-gonic/gin"
)

type S3Handler struct {
	s3Service *services.S3Service
}

func NewS3Handler(s3Service *services.S3Service) *S3Handler {
	return &S3Handler{
		s3Service: s3Service,
	}
}

// UploadFile 上传文件
// @Summary 上传文件
// @Tags File
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "文件"
// @Param folder formData string false "文件夹路径"
// @Success 200 {object} models.UploadResponse
// @Router /api/files/upload [post]
func (h *S3Handler) UploadFile(c *gin.Context) {
	// 获取用户ID（从JWT或session中获取）
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 获取上传的文件
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get file: " + err.Error()})
		return
	}
	defer file.Close()

	// 获取文件夹路径（可选）
	folder := c.DefaultPostForm("folder", "uploads")

	// 上传文件
	fileModel, err := h.s3Service.UploadFile(c.Request.Context(), file, fileHeader, folder, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload file: " + err.Error()})
		return
	}

	response := models.UploadResponse{
		FileID:   fileModel.ID,
		FileName: fileModel.FileName,
		URL:      fileModel.URL,
		S3Key:    fileModel.S3Key,
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file uploaded successfully",
		"data":    response,
	})
}

// GetPresignedURL 获取预签名URL
// @Summary 获取文件预签名URL
// @Tags File
// @Produce json
// @Param id path int true "文件ID"
// @Param expiration query int false "过期时间（秒）" default(3600)
// @Success 200 {object} models.DownloadResponse
// @Router /api/files/{id}/presigned-url [get]
func (h *S3Handler) GetPresignedURL(c *gin.Context) {
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file ID"})
		return
	}

	// 获取文件信息
	file, err := h.s3Service.GetFileByID(c.Request.Context(), uint(fileID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// 获取过期时间
	expirationSeconds := c.DefaultQuery("expiration", "3600")
	expiration, _ := strconv.Atoi(expirationSeconds)

	// 生成预签名URL
	url, err := h.s3Service.GetPresignedURL(c.Request.Context(), file.S3Key, time.Duration(expiration)*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate presigned URL: " + err.Error()})
		return
	}

	response := models.DownloadResponse{
		URL:      url,
		FileName: file.FileName,
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "presigned URL generated successfully",
		"data":    response,
	})
}

// DownloadFile 下载文件
// @Summary 下载文件
// @Tags File
// @Produce application/octet-stream
// @Param id path int true "文件ID"
// @Success 200
// @Router /api/files/{id}/download [get]
func (h *S3Handler) DownloadFile(c *gin.Context) {
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file ID"})
		return
	}

	// 获取文件信息
	file, err := h.s3Service.GetFileByID(c.Request.Context(), uint(fileID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// 下载文件
	data, err := h.s3Service.DownloadFile(c.Request.Context(), file.S3Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to download file: " + err.Error()})
		return
	}

	// 设置响应头
	c.Header("Content-Disposition", "attachment; filename="+file.FileName)
	c.Header("Content-Type", file.FileType)
	c.Data(http.StatusOK, file.FileType, data)
}

// DeleteFile 删除文件
// @Summary 删除文件（软删除）
// @Tags File
// @Produce json
// @Param id path int true "文件ID"
// @Success 200
// @Router /api/files/{id} [delete]
func (h *S3Handler) DeleteFile(c *gin.Context) {
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file ID"})
		return
	}

	if err := h.s3Service.DeleteFile(c.Request.Context(), uint(fileID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted successfully"})
}

// DeleteFilePermanently 永久删除文件
// @Summary 永久删除文件
// @Tags File
// @Produce json
// @Param id path int true "文件ID"
// @Success 200
// @Router /api/files/{id}/permanent [delete]
func (h *S3Handler) DeleteFilePermanently(c *gin.Context) {
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file ID"})
		return
	}

	if err := h.s3Service.DeleteFilePermanently(c.Request.Context(), uint(fileID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file permanently: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted permanently"})
}

// ListFiles 列出文件
// @Summary 列出文件
// @Tags File
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200
// @Router /api/files [get]
func (h *S3Handler) ListFiles(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	files, total, err := h.s3Service.ListFiles(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "files retrieved successfully",
		"data": gin.H{
			"files":     files,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetFileInfo 获取文件信息
// @Summary 获取文件信息
// @Tags File
// @Produce json
// @Param id path int true "文件ID"
// @Success 200
// @Router /api/files/{id} [get]
func (h *S3Handler) GetFileInfo(c *gin.Context) {
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file ID"})
		return
	}

	file, err := h.s3Service.GetFileByID(c.Request.Context(), uint(fileID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file info retrieved successfully",
		"data":    file,
	})
}
