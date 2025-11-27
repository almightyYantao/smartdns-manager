@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion

REM SmartDNS Admin 一键启动开发脚本 (Windows版)
REM 使用方法: dev-start.bat

echo.
echo ========================================
echo   SmartDNS Admin 开发环境启动脚本
echo ========================================
echo.

REM 检查依赖
echo [INFO] 检查依赖...

REM 检查Go
go version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go 未安装，请先安装 Go
    pause
    exit /b 1
)

REM 检查Node.js
node --version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Node.js 未安装，请先安装 Node.js
    pause
    exit /b 1
)

REM 检查npm
npm --version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] npm 未安装，请先安装 npm
    pause
    exit /b 1
)

echo [SUCCESS] 依赖检查完成

REM 检查并安装前端依赖
if not exist "ui\node_modules" (
    echo [INFO] 安装前端依赖...
    cd ui
    npm install
    if errorlevel 1 (
        echo [ERROR] 前端依赖安装失败
        pause
        exit /b 1
    )
    cd ..
    echo [SUCCESS] 前端依赖安装完成
) else (
    echo [INFO] 前端依赖已存在，跳过安装
)

echo.
echo [INFO] 🚀 启动开发环境...
echo.

REM 启动后端
echo [INFO] 启动后端服务...
cd backend
start "SmartDNS Backend" cmd /k "go run main.go"
cd ..
echo [SUCCESS] 后端服务已启动

REM 等待后端启动
timeout /t 3 /nobreak >nul

REM 启动前端
echo [INFO] 启动前端服务...
cd ui
start "SmartDNS Frontend" cmd /k "npm start"
cd ..
echo [SUCCESS] 前端服务已启动

echo.
echo [SUCCESS] 🎉 开发环境启动完成!
echo [INFO] 后端地址: http://localhost:8080
echo [INFO] 前端地址: http://localhost:3000
echo.
echo 两个命令行窗口已打开，分别运行后端和前端服务
echo 关闭对应的命令行窗口即可停止服务
echo.
pause