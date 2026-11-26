package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

var coreUpgradeLock sync.Mutex

func CheckNodeCores(c *gin.Context) {
	release, _ := getCoreRelease(c)
	var nodes []models.Node
	if err := database.DB.Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取节点失败"})
		return
	}
	results := make(map[uint]services.CoreStatus)
	var workers sync.WaitGroup
	var resultsLock sync.Mutex
	limit := make(chan struct{}, 4)
	for _, node := range nodes {
		if c.Request.Context().Err() != nil {
			break
		}
		limit <- struct{}{}
		workers.Add(1)
		go func(node models.Node) {
			defer workers.Done()
			defer func() { <-limit }()
			status := services.CoreStatus{Latest: release.Tag, State: "检测失败"}
			client, err := services.NewSSHClient(&node)
			if err == nil {
				detected, detectErr := client.DetectCore(release)
				client.Close()
				if detectErr == nil {
					status = detected
				}
			}
			resultsLock.Lock()
			results[node.ID] = status
			resultsLock.Unlock()
		}(node)
	}
	workers.Wait()

	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func UpgradeNodeCore(c *gin.Context) {
	var input struct {
		Current    string `json:"current" binding:"required"`
		Target     string `json:"target" binding:"required"`
		Domain     string `json:"domain" binding:"required"`
		ExpectedIP string `json:"expected_ip" binding:"required"`
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 || c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "升级参数无效"})
		return
	}
	if !coreUpgradeLock.TryLock() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "已有节点正在升级"})
		return
	}
	defer coreUpgradeLock.Unlock()
	var node models.Node
	if err := database.DB.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "读取升级节点失败"})
		return
	}
	release, err := getCoreRelease(c)
	if err != nil || release.Tag != input.Target {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "无法确认升级目标版本，请重新检测"})
		return
	}
	client, err := services.NewSSHClient(&node)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "无法通过 SSH 连接节点"})
		return
	}
	defer client.Close()
	current, err := client.DetectCore(release)
	if err != nil || current.Tag != input.Current || !current.CanUpgrade {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "节点升级条件已变化，请重新检测"})
		return
	}
	output, err := client.UpgradeCore(c.Request.Context(), release, current, strings.TrimSpace(input.Domain), strings.TrimSpace(input.ExpectedIP))
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "message": output})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": output})
}

func getCoreRelease(c *gin.Context) (services.CoreRelease, error) {
	source, err := services.LoadCoreSource()
	if err != nil {
		return services.CoreRelease{}, err
	}
	return source.Latest(c.Request.Context())
}

func GetCoreSource(c *gin.Context) {
	source, err := services.LoadCoreSource()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取发布源配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": source})
}

func UpdateCoreSource(c *gin.Context) {
	var source services.CoreSource
	if err := c.ShouldBindJSON(&source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "发布源参数无效"})
		return
	}
	source.MetadataURL = strings.TrimSpace(source.MetadataURL)
	source.AssetBaseURL = strings.TrimSpace(source.AssetBaseURL)
	release, err := source.Latest(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "message": "发布源校验失败，请检查地址和控制台网络"})
		return
	}
	// 保存前验证至少有一个支持架构的实际升级包。
	if err := release.CheckAvailableAsset(c.Request.Context()); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"success": false, "message": "无法下载并校验升级包，请检查下载目录和 SHA-256"})
		return
	}
	if err := services.SaveCoreSource(source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存发布源失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": release.Tag})
}
