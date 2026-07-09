#!/bin/bash
# =============================================================================
# Sub2API 一键启动脚本
# =============================================================================
# 用法: ./start.sh [命令]
#   ./start.sh         启动服务
#   ./start.sh stop    停止服务
#   ./start.sh restart 重启服务
#   ./start.sh status  查看状态
#   ./start.sh logs    查看日志
# =============================================================================

set -e

# -- 颜色 --
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# -- 路径 --
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="${SCRIPT_DIR}/deploy"
COMPOSE_FILE="docker-compose.local.yml"
ENV_FILE=".env"

# -- 打印函数 --
info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# -- 检查依赖 --
check_prerequisites() {
    if ! command -v docker >/dev/null 2>&1; then
        error "Docker 未安装，请先安装 Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! docker info >/dev/null 2>&1; then
        error "Docker 未运行，请先启动 Docker"
        exit 1
    fi

    if docker compose version >/dev/null 2>&1; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose >/dev/null 2>&1; then
        COMPOSE_CMD="docker-compose"
    else
        error "Docker Compose 未安装"
        exit 1
    fi
}

# -- 初始化 .env --
init_env() {
    cd "$DEPLOY_DIR"

    if [ ! -f "$ENV_FILE" ]; then
        if [ ! -f ".env.example" ]; then
            error "找不到 .env.example"
            exit 1
        fi

        info "首次运行，正在初始化配置..."

        cp .env.example .env

        # 生成安全密钥
        local jwt_secret pg_password totp_key
        jwt_secret=$(openssl rand -hex 32)
        pg_password=$(openssl rand -hex 16)
        totp_key=$(openssl rand -hex 32)

        if sed --version >/dev/null 2>&1 2>&1; then
            sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${jwt_secret}|" .env
            sed -i "s|^TOTP_ENCRYPTION_KEY=.*|TOTP_ENCRYPTION_KEY=${totp_key}|" .env
            sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${pg_password}|" .env
        else
            sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=${jwt_secret}|" .env
            sed -i '' "s|^TOTP_ENCRYPTION_KEY=.*|TOTP_ENCRYPTION_KEY=${totp_key}|" .env
            sed -i '' "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${pg_password}|" .env
        fi

        chmod 600 .env

        echo ""
        success "配置文件已生成 (.env)"
        echo -e "  ${CYAN}POSTGRES_PASSWORD:${NC}     ${pg_password}"
        echo -e "  ${CYAN}JWT_SECRET:${NC}            ${jwt_secret}"
        echo -e "  ${CYAN}TOTP_ENCRYPTION_KEY:${NC}   ${totp_key}"
        echo ""
        warn "请妥善保管以上密钥，已写入 .env 文件"
        echo ""
    fi
}

# -- 创建数据目录 --
init_dirs() {
    mkdir -p data postgres_data redis_data
}

# -- 启动 --
do_start() {
    check_prerequisites
    init_env
    init_dirs

    info "正在启动 Sub2API..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" up -d

    echo ""
    success "服务已启动"
    echo ""
    echo -e "  ${CYAN}Web UI:${NC}  http://localhost:${SERVER_PORT:-8080}"
    echo -e "  ${CYAN}日志:${NC}    $COMPOSE_CMD -f $COMPOSE_FILE logs -f sub2api"
    echo -e "  ${CYAN}停止:${NC}    $0 stop"
    echo ""
}

# -- 停止 --
do_stop() {
    check_prerequisites
    cd "$DEPLOY_DIR"
    info "正在停止 Sub2API..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" down
    success "服务已停止"
}

# -- 重启 --
do_restart() {
    check_prerequisites
    cd "$DEPLOY_DIR"
    info "正在重启 Sub2API..."
    $COMPOSE_CMD -f "$COMPOSE_FILE" restart
    success "服务已重启"
}

# -- 状态 --
do_status() {
    check_prerequisites
    cd "$DEPLOY_DIR"
    $COMPOSE_CMD -f "$COMPOSE_FILE" ps
}

# -- 日志 --
do_logs() {
    check_prerequisites
    cd "$DEPLOY_DIR"
    $COMPOSE_CMD -f "$COMPOSE_FILE" logs -f sub2api
}

# -- 主入口 --
case "${1:-}" in
    stop)    do_stop ;;
    restart) do_restart ;;
    status)  do_status ;;
    logs)    do_logs ;;
    *)       do_start ;;
esac
