package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"smartdns-manager/models"
	"smartdns-manager/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SchedulerHandler struct {
	schedulerService *services.SchedulerService
}

func NewSchedulerHandler(schedulerService *services.SchedulerService) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerService: schedulerService,
	}
}

// GetTasks 获取任务列表
func (h *SchedulerHandler) GetTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	taskType := c.Query("type")
	status := c.Query("status")

	offset := (page - 1) * pageSize

	tasks, total, err := h.schedulerService.GetTasks(offset, pageSize, taskType, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询任务失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"tasks":    tasks,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
		"success": true,
	})
}

// CreateTask 创建任务
func (h *SchedulerHandler) CreateTask(c *gin.Context) {
	var req models.ScheduledTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证cron表达式
	if req.CronExpr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Cron表达式不能为空",
		})
		return
	}

	// 创建任务
	if err := h.schedulerService.CreateTask(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建任务失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    req,
		"message": "任务创建成功",
		"success": true,
	})
}

// UpdateTask 更新任务
func (h *SchedulerHandler) UpdateTask(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var req models.ScheduledTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	req.ID = uint(taskID)

	// 更新任务
	if err := h.schedulerService.UpdateTask(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新任务失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    req,
		"message": "任务更新成功",
		"success": true,
	})
}

// DeleteTask 删除任务
func (h *SchedulerHandler) DeleteTask(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 删除任务
	if err := h.schedulerService.DeleteTask(uint(taskID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除任务失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务删除成功",
		"success": true,
	})
}

// GetTask 获取单个任务详情
func (h *SchedulerHandler) GetTask(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	task, err := h.schedulerService.GetTask(uint(taskID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "任务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    task,
		"success": true,
	})
}

// GetTaskExecutions 获取任务执行历史
func (h *SchedulerHandler) GetTaskExecutions(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	offset := (page - 1) * pageSize

	executions, total, err := h.schedulerService.GetTaskExecutions(uint(taskID), offset, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询执行历史失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"executions": executions,
			"total":      total,
			"page":       page,
			"pageSize":   pageSize,
		},
		"success": true,
	})
}

// GetStats 获取调度器统计信息
func (h *SchedulerHandler) GetStats(c *gin.Context) {
	stats, err := h.schedulerService.GetTaskStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取统计信息失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    stats,
		"success": true,
	})
}

// GetRunningTasks 获取正在运行的任务
func (h *SchedulerHandler) GetRunningTasks(c *gin.Context) {
	runningTaskIDs := h.schedulerService.GetRunningTasks()

	var tasks []models.ScheduledTask
	if len(runningTaskIDs) > 0 {
		h.schedulerService.GetDB().Where("id IN ?", runningTaskIDs).Find(&tasks)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    tasks,
		"success": true,
	})
}

// ToggleTask 启用/禁用任务
func (h *SchedulerHandler) ToggleTask(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	task, err := h.schedulerService.GetTask(uint(taskID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "任务不存在",
		})
		return
	}

	// 切换启用状态
	task.Enabled = !task.Enabled

	// 更新任务
	if err := h.schedulerService.UpdateTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新任务状态失败",
			"error":   err.Error(),
		})
		return
	}

	action := "启用"
	if !task.Enabled {
		action = "禁用"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    task,
		"message": "任务" + action + "成功",
		"success": true,
	})
}

// CreateQuickTask 创建快速任务（预定义模板）
func (h *SchedulerHandler) CreateQuickTask(c *gin.Context) {
	var req struct {
		Type   string          `json:"type" binding:"required"`
		Name   string          `json:"name" binding:"required"`
		Cron   string          `json:"cron" binding:"required"`
		Config json.RawMessage `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	task := models.ScheduledTask{
		Name:        req.Name,
		Type:        models.TaskType(req.Type),
		CronExpr:    req.Cron,
		Config:      string(req.Config),
		Enabled:     true,
		Description: "快速创建的任务",
	}

	// 创建任务
	if err := h.schedulerService.CreateTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建任务失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    task,
		"message": "任务创建成功",
		"success": true,
	})
}

// ExecuteTask 手动执行任务
func (h *SchedulerHandler) ExecuteTask(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 获取任务信息
	task, err := h.schedulerService.GetTask(uint(taskID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "任务不存在",
		})
		return
	}

	// 检查任务是否已在执行
	runningTasks := h.schedulerService.GetRunningTasks()
	for _, runningID := range runningTasks {
		if runningID == uint(taskID) {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": "任务正在执行中，请稍后再试",
			})
			return
		}
	}

	// 使用调度服务执行任务
	err = h.schedulerService.ExecuteTaskManually(*task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "执行任务失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已开始执行",
		"success": true,
	})
}

// GetTelemetryTargets 获取遥测目标列表
func (h *SchedulerHandler) GetTelemetryTargets(c *gin.Context) {
	var targets []models.TelemetryTarget

	if err := h.schedulerService.GetDB().Find(&targets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询遥测目标失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    targets,
		"success": true,
	})
}

// CreateTelemetryTarget 创建遥测目标
func (h *SchedulerHandler) CreateTelemetryTarget(c *gin.Context) {
	var target models.TelemetryTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	if err := h.schedulerService.GetDB().Create(&target).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建遥测目标失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    target,
		"message": "遥测目标创建成功",
		"success": true,
	})
}

// UpdateTelemetryTarget 更新遥测目标
func (h *SchedulerHandler) UpdateTelemetryTarget(c *gin.Context) {
	targetID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var target models.TelemetryTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	target.ID = uint(targetID)

	if err := h.schedulerService.GetDB().Save(&target).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新遥测目标失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    target,
		"message": "遥测目标更新成功",
		"success": true,
	})
}

// DeleteTelemetryTarget 删除遥测目标
func (h *SchedulerHandler) DeleteTelemetryTarget(c *gin.Context) {
	targetID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	if err := h.schedulerService.GetDB().Delete(&models.TelemetryTarget{}, targetID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除遥测目标失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "遥测目标删除成功",
		"success": true,
	})
}

// GetTelemetryResults 获取遥测结果
func (h *SchedulerHandler) GetTelemetryResults(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	targetID := c.Query("target_id")

	offset := (page - 1) * pageSize

	var results []models.TelemetryResult
	query := h.schedulerService.GetDB().
		Order("checked_at DESC").
		Offset(offset).
		Limit(pageSize)

	if targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}

	if err := query.Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询遥测结果失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应结构体，包含目标名称
	type ResultWithName struct {
		models.TelemetryResult
		TargetName string `json:"target_name"`
	}
	
	var resultsWithNames []ResultWithName
	for _, result := range results {
		var target models.TelemetryTarget
		resultWithName := ResultWithName{
			TelemetryResult: result,
		}
		if err := h.schedulerService.GetDB().First(&target, result.TargetID).Error; err == nil {
			resultWithName.TargetName = target.Name
		}
		resultsWithNames = append(resultsWithNames, resultWithName)
	}

	var total int64
	countQuery := h.schedulerService.GetDB().Model(&models.TelemetryResult{})
	if targetID != "" {
		countQuery = countQuery.Where("target_id = ?", targetID)
	}
	countQuery.Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"results":  resultsWithNames,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
		"success": true,
	})
}

// GetTelemetryStats 获取遥测统计信息
func (h *SchedulerHandler) GetTelemetryStats(c *gin.Context) {
	var stats struct {
		TotalTargets  int64   `json:"total_targets"`
		OnlineTargets int64   `json:"online_targets"`
		AvgLatency    float64 `json:"avg_latency"`
		SuccessRate   float64 `json:"success_rate"`
	}

	// 总目标数
	h.schedulerService.GetDB().Model(&models.TelemetryTarget{}).Count(&stats.TotalTargets)

	// 在线目标数（最近一次检查成功的）
	h.schedulerService.GetDB().Model(&models.TelemetryTarget{}).
		Where("last_status = ?", true).
		Count(&stats.OnlineTargets)

	// 平均延迟（最近24小时的成功结果）
	var avgResult struct {
		AvgLatency float64 `json:"avg_latency"`
	}
	h.schedulerService.GetDB().
		Model(&models.TelemetryResult{}).
		Select("AVG(latency) as avg_latency").
		Where("success = ? AND checked_at > datetime('now', '-24 hours')", true).
		Scan(&avgResult)
	stats.AvgLatency = avgResult.AvgLatency

	// 成功率（最近24小时）
	var totalChecks, successChecks int64
	h.schedulerService.GetDB().
		Model(&models.TelemetryResult{}).
		Where("checked_at > datetime('now', '-24 hours')").
		Count(&totalChecks)

	if totalChecks > 0 {
		h.schedulerService.GetDB().
			Model(&models.TelemetryResult{}).
			Where("success = ? AND checked_at > datetime('now', '-24 hours')", true).
			Count(&successChecks)
		stats.SuccessRate = float64(successChecks) / float64(totalChecks)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    stats,
		"success": true,
	})
}

// TestTelemetryTarget 测试遥测目标
func (h *SchedulerHandler) TestTelemetryTarget(c *gin.Context) {
	targetID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// 获取目标信息
	var target models.TelemetryTarget
	if err := h.schedulerService.GetDB().First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "遥测目标不存在",
		})
		return
	}

	// 获取遥测服务
	telemetryService := h.schedulerService.GetTelemetryService()
	if telemetryService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "遥测服务未初始化",
		})
		return
	}

	// 执行真实的网络检测
	ctx := c.Request.Context()
	result, err := telemetryService.CheckSingleTarget(ctx, target)
	
	if err != nil {
		// 即使检测失败，也要保存结果
		if result == nil {
			result = &models.TelemetryResult{
				TargetID:  target.ID,
				CheckedAt: time.Now(),
				Success:   false,
				Error:     err.Error(),
			}
		}
	}

	// 保存结果到数据库
	if err := h.schedulerService.GetDB().Create(result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存测试结果失败",
			"error":   err.Error(),
		})
		return
	}

	// 更新目标的最后检查信息
	updates := map[string]interface{}{
		"last_check_at": &result.CheckedAt,
		"last_status":   result.Success,
		"last_latency":  result.Latency,
		"check_count":   gorm.Expr("check_count + 1"),
	}
	
	if result.Success {
		updates["success_count"] = gorm.Expr("success_count + 1")
		
		// 计算平均延迟
		var avgLatency float64
		h.schedulerService.GetDB().Model(&models.TelemetryResult{}).
			Where("target_id = ? AND success = ?", target.ID, true).
			Select("AVG(latency)").Scan(&avgLatency)
		updates["avg_latency"] = avgLatency
	}
	
	h.schedulerService.GetDB().Model(&target).Updates(updates)

	// 重新加载目标信息以包含更新后的统计数据
	if err := h.schedulerService.GetDB().First(&target, targetID).Error; err == nil {
		result.Target = target
	}

	message := "测试完成"
	if result.Success {
		message = fmt.Sprintf("测试成功，延迟: %dms", result.Latency)
	} else {
		message = fmt.Sprintf("测试失败: %s", result.Error)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    result,
		"message": message,
		"success": true,
	})
}

// GetScriptTemplates 获取脚本模板列表
func (h *SchedulerHandler) GetScriptTemplates(c *gin.Context) {
	templates := h.schedulerService.GetCustomScriptService().GetScriptTemplates()
	
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    templates,
		"success": true,
	})
}
