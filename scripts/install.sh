#!/bin/sh
# OQM Installation Script for OpenWrt

set -e

VERSION="1.0.0"
INSTALL_DIR="/usr/bin"
CONFIG_DIR="/etc/oqm"
BINARY_NAME="oqm"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect architecture
detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64)
            BINARY_ARCH="amd64"
            ;;
        armv7l|armhf)
            BINARY_ARCH="arm"
            ;;
        aarch64|arm64)
            BINARY_ARCH="arm64"
            ;;
        mips)
            BINARY_ARCH="mips"
            ;;
        mipsel)
            BINARY_ARCH="mipsle"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    info "Detected architecture: $BINARY_ARCH"
}

# Check for nftables
check_dependencies() {
    info "Checking dependencies..."

    if ! command -v nft >/dev/null 2>&1; then
        error "nftables is not installed. Please install it first: opkg install nftables"
    fi

    info "All dependencies satisfied"
}

# Download binary
download_binary() {
    info "Downloading OQM v${VERSION} for ${BINARY_ARCH}..."

    DOWNLOAD_URL="https://github.com/abdollahzadehghalejoghi/oqm/releases/download/v${VERSION}/${BINARY_NAME}-${BINARY_ARCH}"

    if command -v wget >/dev/null 2>&1; then
        wget -O "/tmp/${BINARY_NAME}" "$DOWNLOAD_URL" || error "Failed to download binary"
    elif command -v curl >/dev/null 2>&1; then
        curl -L -o "/tmp/${BINARY_NAME}" "$DOWNLOAD_URL" || error "Failed to download binary"
    else
        error "Neither wget nor curl is available. Please install one of them."
    fi

    info "Download complete"
}

# Install binary
install_binary() {
    info "Installing OQM binary..."

    chmod +x "/tmp/${BINARY_NAME}"
    mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

    info "Binary installed to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Create directories
create_directories() {
    info "Creating directories..."

    mkdir -p "$CONFIG_DIR"

    info "Directories created"
}

# Initialize configuration
init_config() {
    info "Initializing configuration..."

    if [ -f "${CONFIG_DIR}/data.json" ]; then
        warn "Configuration file already exists. Skipping initialization."
    else
        cat > "${CONFIG_DIR}/data.json" <<EOF
{
  "users": [],
  "config": {
    "check_interval": 60,
    "web_port": 8080,
    "bot_type": "telegram",
    "bot_token": "",
    "bot_api_base_url": "",
    "admin_chat_id": "",
    "reset_schedule": "monthly",
    "nftables_table": "oqm",
    "data_dir": "/etc/oqm",
    "log_file": "/var/log/oqm.log"
  }
}
EOF
        info "Configuration initialized"
    fi
}

# Create init script
create_init_script() {
    info "Creating init script..."

    cat > /etc/init.d/oqm <<'EOF'
#!/bin/sh /etc/rc.common

START=95
STOP=10

USE_PROCD=1

PROG=/usr/bin/oqm
CONF_DIR=/etc/oqm

start_service() {
    procd_open_instance
    procd_set_param command $PROG daemon run
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param env OQM_DATA_DIR=$CONF_DIR
    procd_close_instance

    procd_open_instance web
    procd_set_param command $PROG web
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_set_param env OQM_DATA_DIR=$CONF_DIR
    procd_close_instance
}

stop_service() {
    killall oqm
}

reload_service() {
    stop
    start
}
EOF

    chmod +x /etc/init.d/oqm
    info "Init script created"
}

# Enable and start service
enable_service() {
    info "Enabling OQM service..."

    /etc/init.d/oqm enable
    /etc/init.d/oqm start

    info "OQM service enabled and started"
}

# Main installation
main() {
    info "Starting OQM installation..."
    echo ""

    detect_arch
    check_dependencies
    download_binary
    install_binary
    create_directories
    init_config
    create_init_script
    enable_service

    echo ""
    info "Installation complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Configure bot notifications (optional):"
    echo "     - For Telegram: oqm config set --key bot-type --value telegram"
    echo "                     oqm config set --key bot-token --value YOUR_BOT_TOKEN"
    echo "                     oqm config set --key admin-chat-id --value YOUR_CHAT_ID"
    echo "     - For Bale:     oqm config set --key bot-type --value bale"
    echo "                     oqm config set --key bot-token --value YOUR_BOT_TOKEN"
    echo "                     oqm config set --key admin-chat-id --value YOUR_CHAT_ID"
    echo "  2. Add your first user: oqm add-user --ip 192.168.1.100 --mac XX:XX:XX:XX:XX:XX --name 'User' --quota 5120"
    echo "  3. Access web UI: http://$(uci get network.lan.ipaddr):8080"
    echo ""
}

# Run installation
main
