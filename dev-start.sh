#!/bin/bash

# SmartDNS Admin 一键启动开发脚本
# 使用方法: ./dev-start.sh

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    print_info "检查依赖..."
    
    # 检查Go
    if ! command -v go &> /dev/null; then
        print_error "Go 未安装，请先安装 Go"
        exit 1
    fi
    
    # 检查Node.js
    if ! command -v node &> /dev/null; then
        print_error "Node.js 未安装，请先安装 Node.js"
        exit 1
    fi
    
    # 检查npm
    if ! command -v npm &> /dev/null; then
        print_error "npm 未安装，请先安装 npm"
        exit 1
    fi
    
    print_success "依赖检查完成"
}

# 安装前端依赖
install_frontend_deps() {
    if [ ! -d "ui/node_modules" ]; then
        print_info "安装前端依赖..."
        cd ui
        npm install
        cd ..
        print_success "前端依赖安装完成"
    else
        print_info "前端依赖已存在，跳过安装"
    fi
}

# 清理函数
cleanup() {
    print_warning "正在停止所有进程..."
    if [ ! -z "$BACKEND_PID" ]; then
        kill $BACKEND_PID 2>/dev/null || true
    fi
    if [ ! -z "$FRONTEND_PID" ]; then
        kill $FRONTEND_PID 2>/dev/null || true
    fi
    exit 0
}

# 设置信号处理
trap cleanup SIGINT SIGTERM

# 主函数
main() {
    print_info "🚀 启动 SmartDNS Admin 开发环境..."
    
    # 检查依赖
    check_dependencies
    
    # 安装前端依赖
    install_frontend_deps
    
    # 启动后端
    print_info "启动后端服务..."
    cd backend
    go run main.go &
    BACKEND_PID=$!
    cd ..
    print_success "后端服务已启动 (PID: $BACKEND_PID)"
    
    # 等待一下让后端启动
    sleep 3
    
    # 启动前端
    print_info "启动前端服务..."
    cd ui
    npm start &
    FRONTEND_PID=$!
    cd ..
    print_success "前端服务已启动 (PID: $FRONTEND_PID)"
    
    print_success "🎉 开发环境启动完成!"
    print_info "后端地址: http://localhost:8080"
    print_info "前端地址: http://localhost:3000"
    print_warning "按 Ctrl+C 停止所有服务"
    
    # 等待进程
    wait
}

# 运行主函数
main