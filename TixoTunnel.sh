#!/usr/bin/env bash
set -o pipefail

SCRIPT_VERSION="NEXUS-1.0.0"
ENGINE_EDITION="AETHER-X1"
BRAND_NAME="TixoTunnel"
BRAND_CHANNEL="@TixoCloud"
BRAND_WEBSITE="TixoCloud.com"
GITHUB_REPO="TheTixoCloud/TixoTunnel"

INSTALL_DIR="/root/tixotunnel-core"
PANEL_PATH="/root/TixoTunnel.sh"
COMMAND_PATH="/usr/local/bin/tixotunnel"
CORE_DOWNLOAD_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main/core/tixotunnel-core"
SPOOF_TESTER_DOWNLOAD_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main/core/tixotunnel-core.engine"
SPOOF_TESTER_FALLBACK_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/tixotunnel-core.engine"
SPOOF_TESTER_FILE="${INSTALL_DIR}/tixotunnel-core.engine"
PANEL_DOWNLOAD_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main/TixoTunnel.sh"

bootstrap_install() {
    if [[ ${EUID} -ne 0 ]]; then
        echo "TixoTunnel must be run as root."
        exit 1
    fi

    # Running from curl/process substitution or an arbitrary local path:
    # install the canonical copy first, then continue from it.
    local current_path="${BASH_SOURCE[0]:-}"
    if [[ "$current_path" != "$PANEL_PATH" && "$current_path" != "$COMMAND_PATH" ]]; then
        command -v curl >/dev/null 2>&1 || {
            apt-get update -y && apt-get install -y curl ca-certificates
        }

        mkdir -p "$INSTALL_DIR"
        local tmp_panel tmp_core tmp_tester
        tmp_panel=$(mktemp)
        tmp_core=$(mktemp)
        tmp_tester=$(mktemp)
        trap 'rm -f "$tmp_panel" "$tmp_core" "$tmp_tester"' RETURN

        clear
        printf '\033[38;5;196m%s\033[0m\n' '___________.__             _________ .__                   .___'
        printf '\033[38;5;196m%s\033[0m\n' '\__    ___/|__|__  _______ \_   ___ \|  |   ____  __ __  __| _/'
        printf '\033[38;5;196m%s\033[0m\n' '  |    |   |  \  \/  /  _ \/    \  \/|  |  /  _ \|  |  \/ __ |'
        printf '\033[38;5;196m%s\033[0m\n' '  |    |   |  |>    <  <_> )     \___|  |_(  <_> )  |  / /_/ |'
        printf '\033[38;5;196m%s\033[0m\n' '  |____|   |__/__/\_ \____/ \______  /____/\____/|____/\____ |'
        printf '\033[38;5;196m%s\033[0m\n' '                    \/             \/                       \/'
        printf '\033[38;5;245m%s\033[0m\n' '════════════════════════════════════════════════════════════════════'
        printf '\033[97m Console    \033[38;5;51m%-15s \033[97mEngine     \033[38;5;46m%s\033[0m\n' "$SCRIPT_VERSION" "$ENGINE_EDITION"
        printf '\033[97m Status     \033[38;5;46m%-15s \033[97mChannel    \033[38;5;51m%s\033[0m\n' '● Installing' "$BRAND_CHANNEL"
        printf '\033[97m Features   \033[38;5;245m%s\033[0m\n' 'TUN • IPX • Encryption • Auto Tuning • IP Spoofing'
        printf '\033[38;5;245m%s\033[0m\n\n' '════════════════════════════════════════════════════════════════════'

        echo '[1/5] Downloading TixoTunnel panel...'
        curl -fL --retry 3 --connect-timeout 15 -o "$tmp_panel" "$PANEL_DOWNLOAD_URL" || {
            echo 'Panel download failed.'
            exit 1
        }

        echo '[2/5] Downloading TixoTunnel core...'
        curl -fL --retry 3 --connect-timeout 15 -o "$tmp_core" "$CORE_DOWNLOAD_URL" || {
            echo 'Core download failed. Make sure tixotunnel-core exists in the latest GitHub Release.'
            exit 1
        }

        echo '[3/5] Installing Spoof Tester...'
        local local_tester="$(cd "$(dirname "$current_path")" 2>/dev/null && pwd)/core/tixotunnel-core.engine"
        if [[ -f "$local_tester" ]]; then
            cp -f "$local_tester" "$tmp_tester"
        else
            if ! curl -fL --retry 3 --connect-timeout 15 -o "$tmp_tester" "$SPOOF_TESTER_DOWNLOAD_URL"; then
                echo 'Release asset unavailable; trying repository fallback...'
                curl -fL --retry 3 --connect-timeout 15 -o "$tmp_tester" "$SPOOF_TESTER_FALLBACK_URL" || {
                    echo 'Spoof Tester download failed from all configured sources.'
                    : > "$tmp_tester"
                }
            fi
        fi

        echo '[4/5] Creating directories and setting permissions...'

        install -m 0755 "$tmp_panel" "$PANEL_PATH"
        install -m 0755 "$tmp_panel" "$COMMAND_PATH"
        install -m 0755 "$tmp_core" "$INSTALL_DIR/tixotunnel-core"
        [[ -s "$tmp_tester" ]] && install -m 0755 "$tmp_tester" "$SPOOF_TESTER_FILE"

        echo '[5/5] Installation completed.'
        echo 'Run later with: tixotunnel'
        sleep 1
        exec "$PANEL_PATH"
    fi

    mkdir -p "$INSTALL_DIR"
    chmod 0755 "$PANEL_PATH" "$COMMAND_PATH" 2>/dev/null || true
}

bootstrap_install

service_dir="/etc/systemd/system"
config_dir="/root/tixotunnel-core"
CORE_FILE="${config_dir}/tixotunnel-core"
SPOOF_TESTER_FILE="${config_dir}/tixotunnel-core.engine"
SPOOF_TEST_DIR="${config_dir}/spoof-tests"
LINK_TEST_DIR="${config_dir}/link-benchmarks"
DIAG_DIR="${config_dir}/diagnostics"
SNAPSHOT_DIR="${config_dir}/snapshots"
BACKUP_DIR="${config_dir}/backups"
STRESS_DIR="${config_dir}/stress-tests"
SPEEDTEST_DIR="${config_dir}/server-speedtests"
UPDATE_BACKUP_DIR="/root/tixotunnel-update-backups"
CERT_DIR="${config_dir}/cert_files"
CERT_FILE="${CERT_DIR}/cert.crt"
KEY_FILE="${CERT_DIR}/cert.key"
mkdir -p "$CERT_DIR"
if [[ $EUID -ne 0 ]]; then
echo "This script must be run as root"
sleep 1
exit 1
fi
colorize() {
local color="$1"
local text="$2"
local style="${3:-normal}"
local black="\033[30m" red="\033[31m" green="\033[32m" yellow="\033[33m"
local blue="\033[34m" magenta="\033[35m" cyan="\033[36m" white="\033[37m"
local reset="\033[0m" normal="\033[0m" bold="\033[1m" underline="\033[4m"
local color_code
case $color in
black) color_code=$black ;; red) color_code=$red ;;
green) color_code=$green ;; yellow) color_code=$yellow ;;
blue) color_code=$blue ;; magenta) color_code=$magenta ;;
cyan) color_code=$cyan ;; white) color_code=$white ;;
*) color_code=$reset ;;
esac
local style_code
case $style in
bold) style_code=$bold ;; underline) style_code=$underline ;;
normal | *) style_code=$normal ;;
esac
echo -e "${style_code}${color_code}${text}${reset}"
}
section_header() {
local title="$1"
echo -e "\033[38;5;245m────────────────────────────────────────────────────────────────────\033[0m"
echo -e "\033[97m ${title}\033[0m"
echo -e "\033[38;5;245m────────────────────────────────────────────────────────────────────\033[0m"
}
wizard_header() {
local step="$1" title="$2" subtitle="${3:-}" current total filled empty bar
current="${step%%/*}"; total="${step##*/}"
if [[ "$current" =~ ^[0-9]+$ && "$total" =~ ^[0-9]+$ && "$total" -gt 0 ]]; then
  filled=$(( current * 20 / total )); empty=$((20-filled))
  bar="$(printf '%*s' "$filled" '' | tr ' ' '■')$(printf '%*s' "$empty" '' | tr ' ' '□')"
else bar=""; fi
echo -e "\033[38;5;245m────────────────────────────────────────────────────────────────────\033[0m"
printf "\033[38;5;51m STEP %-5s\033[0m  \033[97m%s\033[0m\n" "$step" "$title"
[[ -n "$bar" ]] && printf " \033[38;5;51m[%s]\033[0m %d%%\n" "$bar" "$((current*100/total))"
[[ -n "$subtitle" ]] && printf " \033[38;5;245m%s\033[0m\n" "$subtitle"
echo -e "\033[38;5;245m────────────────────────────────────────────────────────────────────\033[0m"
}
select_option() {
local prompt="$1" default="$2" var_name="$3"; shift 3
local options=("$@") choice i
for i in "${!options[@]}"; do
    printf "  \033[38;5;51m[%d]\033[0m %s\n" "$((i+1))" "${options[$i]}"
done
while true; do
    read -r -p "$prompt [${default}]: " choice
    choice="${choice:-$default}"
    if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= ${#options[@]} )); then
        printf -v "$var_name" '%s' "${options[$((choice-1))]}"
        return 0
    fi
    colorize red "Invalid selection. Choose 1-${#options[@]}."
done
}
press_key() {
read -r -p "Press Enter to continue..."
}
prompt_with_default() {
local prompt="$1"
local default="$2"
local var_name="$3"
local input
echo -ne "[-] $prompt (default: $default): "
read -r input
eval "$var_name=\"${input:-$default}\""
}
prompt_boolean() {
local prompt="$1"
local default="$2"
local var_name="$3"
while true; do
prompt_with_default "$prompt [true/false]" "$default" "$var_name"
local value="${!var_name}"
if [[ "$value" == "true" || "$value" == "false" ]]; then
break
fi
colorize red "Invalid input. Please enter 'true' or 'false'."
done
}
validate_cidr() {
local cidr="$1"
if [[ ! "$cidr" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]{1,2})$ ]]; then
return 1
fi
IFS='/' read -r ip mask <<< "$cidr"
IFS='.' read -r a b c d <<< "$ip"
if (( a<0 || a>255 || b<0 || b>255 || c<0 || c>255 || d<0 || d>255 )); then
return 1
fi
if (( mask < 1 || mask > 32 )); then
return 1
fi
local ip_int=$(( (a << 24) | (b << 16) | (c << 8) | d ))
local mask_int
if (( mask == 32 )); then
mask_int=0xFFFFFFFF
else
mask_int=$(( (0xFFFFFFFF << (32 - mask)) & 0xFFFFFFFF ))
fi
local net_int=$(( ip_int & mask_int ))
local broadcast_int=$(( net_int | (~mask_int & 0xFFFFFFFF) ))
if (( ip_int == net_int )); then
return 1
fi
if (( ip_int == broadcast_int )); then
return 1
fi
return 0
}
valid_ipv4() {
local ip="$1" a b c d
[[ "$ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || return 1
IFS='.' read -r a b c d <<< "$ip"
for octet in "$a" "$b" "$c" "$d"; do
    [[ "$octet" =~ ^[0-9]+$ ]] || return 1
    (( 10#$octet >= 0 && 10#$octet <= 255 )) || return 1
done
return 0
}

generate_shared_key() {
if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -d '\n'
else
    head -c 32 /dev/urandom | base64 | tr -d '\n'
fi
}

prompt_shared_key() {
local input generated
colorize yellow "Use exactly the same shared key on both tunnel endpoints."
echo -ne "[-] Shared Encryption Key (paste existing key, or press Enter to generate): "
read -r input
if [[ -z "$input" ]]; then
    generated=$(generate_shared_key)
    CONFIG[psk]="$generated"
    colorize green "Generated key: $generated" bold
    colorize yellow "Save this key; enter it on the opposite server."
else
    CONFIG[psk]="$input"
fi
}


install_spoof_tester() {
local local_candidate="${1:-}"
mkdir -p "$config_dir" "$SPOOF_TEST_DIR"
if [[ -x "$SPOOF_TESTER_FILE" ]]; then
    return 0
fi
if [[ -n "$local_candidate" && -f "$local_candidate" ]]; then
    install -m 0755 "$local_candidate" "$SPOOF_TESTER_FILE"
    return 0
fi
local script_dir
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)
if [[ -f "$script_dir/core/tixotunnel-core.engine" ]]; then
    install -m 0755 "$script_dir/core/tixotunnel-core.engine" "$SPOOF_TESTER_FILE"
    return 0
fi
local tmp_file
 tmp_file=$(mktemp)
colorize cyan "Downloading Tixo Spoof Tester..." bold
if curl -fL --retry 3 --connect-timeout 15 -o "$tmp_file" "$SPOOF_TESTER_DOWNLOAD_URL" \
   || curl -fL --retry 3 --connect-timeout 15 -o "$tmp_file" "$SPOOF_TESTER_FALLBACK_URL"; then
    if [[ -s "$tmp_file" ]]; then
        install -m 0755 "$tmp_file" "$SPOOF_TESTER_FILE"
        rm -f "$tmp_file"
        colorize green "Spoof Tester downloaded and installed successfully." bold
        return 0
    fi
fi
rm -f "$tmp_file"
colorize red "Automatic Spoof Tester download failed." bold
colorize yellow "Check internet access or publish one of these GitHub assets:"
echo "  Release asset : tixotunnel-core.engine"
echo "  Repository    : core/tixotunnel-core.engine"
return 1
}

valid_positive_int() {
[[ "$1" =~ ^[0-9]+$ ]] && (( 10#$1 > 0 ))
}

prompt_positive_int() {
local prompt="$1" default="$2" var_name="$3" value
while true; do
    read -r -p "$prompt [$default]: " value
    value="${value:-$default}"
    if valid_positive_int "$value"; then
        printf -v "$var_name" '%s' "$value"
        return 0
    fi
    colorize red "Enter a positive whole number."
done
}

prepare_spoof_source_list() {
local output_file="$1" choice input source_file
section_header "Source Address Set"
echo -e "  \033[38;5;51m[1]\033[0m Enter IPs or ranges now"
echo -e "  \033[38;5;51m[2]\033[0m Use an existing file"
echo
echo -e "  \033[38;5;245mAccepted: 192.0.2.10 · 192.0.2.1-192.0.2.254 · 192.0.2.0/24\033[0m"
read -r -p "Input method [1]: " choice
choice="${choice:-1}"
case "$choice" in
    1)
        echo
        echo "Enter one or more addresses/ranges separated by commas or spaces."
        read -r -p "Source set: " input
        [[ -n "$input" ]] || { colorize red "Source set cannot be empty."; return 1; }
        printf '%s\n' "$input" | tr ', ' '\n\n' | sed '/^[[:space:]]*$/d' > "$output_file"
        ;;
    2)
        read -r -p "File path: " source_file
        [[ -f "$source_file" ]] || { colorize red "File not found."; return 1; }
        cp -f "$source_file" "$output_file"
        ;;
    *) colorize red "Invalid option."; return 1 ;;
esac
[[ -s "$output_file" ]] || { colorize red "No source entries were loaded."; return 1; }
return 0
}

spoof_tester_run() {
install_spoof_tester || { press_key; return 1; }
mkdir -p "$SPOOF_TEST_DIR"
clear
section_header "Spoof Capability Tester"
echo -e "  Test whether forged source addresses can reach the peer server."
echo -e "  \033[38;5;245mStart Receiver first, then run Sender on the opposite server.\033[0m"
echo
echo -e "  \033[38;5;51m[1]\033[0m Sender   \033[38;5;245mtransmit test packets\033[0m"
echo -e "  \033[38;5;51m[2]\033[0m Receiver \033[38;5;245mcapture and grade received packets\033[0m"
echo -e "  \033[38;5;245m[0] Back\033[0m"
read -r -p "Select tester role [0-2]: " role_choice
case "$role_choice" in
    1) mode="sender" ;;
    2) mode="receiver" ;;
    0) return ;;
    *) colorize red "Invalid option."; sleep 1; return ;;
esac

clear
wizard_header "1/4" "TEST PROTOCOL" "Use ICMP for a simple reachability test or TCP for port-filtered testing"
select_option "Protocol" "1" protocol "icmp" "tcp"
local dst_port=""
if [[ "$protocol" == "tcp" && "$mode" == "receiver" ]]; then
    while true; do
        read -r -p "Filter Port [0 = all]: " dst_port
        dst_port="${dst_port:-0}"
        [[ "$dst_port" =~ ^[0-9]+$ ]] && (( dst_port >= 0 && dst_port <= 65535 )) && break
        colorize red "Enter 0 for all ports or a valid port from 1 to 65535."
    done
elif [[ "$protocol" == "tcp" && "$mode" == "sender" ]]; then
    while true; do
        read -r -p "Destination Port [1-65535]: " dst_port
        [[ "$dst_port" =~ ^[0-9]+$ ]] && (( dst_port >= 1 && dst_port <= 65535 )) && break
        colorize red "Destination Port is required and must be between 1 and 65535."
    done
fi

local stamp src_file log_file passed_file
stamp=$(date +%Y%m%d-%H%M%S)
src_file="$SPOOF_TEST_DIR/sources-$stamp.txt"
log_file="$SPOOF_TEST_DIR/test-$mode-$stamp.log"
passed_file="$SPOOF_TEST_DIR/passed-$stamp.txt"
clear
wizard_header "2/4" "SOURCE RANGE" "Addresses that will be forged and tested"
prepare_spoof_source_list "$src_file" || { rm -f "$src_file"; press_key; return 1; }

local packet_count max_loss timeout concurrency dst_ip
clear
wizard_header "3/4" "TEST POLICY" "Tune attempts, acceptable loss and test duration"
prompt_positive_int "Packets per source IP" "10" packet_count
while true; do
    read -r -p "Maximum packet loss percent [20]: " max_loss
    max_loss="${max_loss:-20}"
    [[ "$max_loss" =~ ^([0-9]+([.][0-9]+)?|[.][0-9]+)$ ]] && awk "BEGIN{exit !($max_loss >= 0 && $max_loss <= 100)}" && break
    colorize red "Enter a number from 0 to 100."
done
prompt_positive_int "Test timeout in seconds" "30" timeout
if [[ "$mode" == "sender" ]]; then
    while true; do
        read -r -p "Peer server public IP: " dst_ip
        valid_ipv4 "$dst_ip" && break
        colorize red "Enter a valid IPv4 address."
    done
    prompt_positive_int "Sender concurrency" "100" concurrency
else
    concurrency="100"
fi

clear
wizard_header "4/4" "EXECUTION REVIEW" "Confirm both nodes use the same protocol, source set and policy"
printf "  %-15s : %s\n" "Role" "${mode^^}"
printf "  %-15s : %s\n" "Protocol" "$protocol"
if [[ "$protocol" == "tcp" && "$mode" == "receiver" ]]; then
    [[ "$dst_port" == 0 ]] && printf "  %-15s : %s\n" "Filter Port" "All ports" || printf "  %-15s : %s\n" "Filter Port" "$dst_port"
elif [[ "$protocol" == "tcp" && "$mode" == "sender" ]]; then
    printf "  %-15s : %s:%s\n" "Destination" "$dst_ip" "$dst_port"
fi
printf "  %-15s : %s\n" "Source entries" "$(wc -l < "$src_file" | tr -d ' ')"
printf "  %-15s : %s\n" "Packets / IP" "$packet_count"
printf "  %-15s : %s%%\n" "Maximum loss" "$max_loss"
printf "  %-15s : %ss\n" "Timeout" "$timeout"
[[ "$mode" == "sender" ]] && printf "  %-15s : %s\n" "Concurrency" "$concurrency"
echo
if [[ "$mode" == "receiver" ]]; then
    colorize yellow "Keep this running, then launch Sender on the peer server."
fi
read -r -p "Start test? [Y/n]: " confirm
[[ "$confirm" =~ ^[Nn]$ ]] && return

echo
colorize cyan "Spoof test is running..." bold
local -a cmd
cmd=("$SPOOF_TESTER_FILE" tester --mode "$mode" --protocol "$protocol" --src-list "$src_file" --timeout "$timeout" --packet-count "$packet_count" --max-loss "$max_loss")
[[ "$protocol" == "tcp" ]] && cmd+=(--dst-port "$dst_port")
if [[ "$mode" == "sender" ]]; then
    cmd+=(--dst-ip "$dst_ip" --concurrency "$concurrency")
fi
"${cmd[@]}" 2>&1 | tee "$log_file"
local rc=${PIPESTATUS[0]}
if (( rc != 0 )); then
    colorize red "Spoof test failed. Review: $log_file"
    press_key
    return "$rc"
fi
if [[ "$mode" == "receiver" ]]; then
    awk '/^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+[[:space:]]+[0-9]+\/[0-9]+[[:space:]]+[0-9.]+%$/ {print $1}' "$log_file" | sort -u > "$passed_file"
    echo
    section_header "Passed Spoof Addresses"
    if [[ -s "$passed_file" ]]; then
        nl -w2 -s') ' "$passed_file"
        echo
        colorize green "Saved list: $passed_file" bold
    else
        colorize yellow "No source address met the selected loss threshold."
    fi
else
    echo
    colorize green "Sender completed. Check Receiver results on the peer server." bold
fi
colorize cyan "Full log: $log_file"
press_key
}

install_jq() {
if ! command -v jq &> /dev/null; then
if command -v apt-get &> /dev/null; then
colorize yellow "Installing jq..."
sudo apt-get update && sudo apt-get install -y jq
else
colorize red "Error: Unsupported package manager. Please install jq manually."
press_key
exit 1
fi
fi
}
download_tixo_engine() {
local source_url="https://github.com/${GITHUB_REPO}/releases/latest/download/tixotunnel-core"
if [[ "$1" == "menu" ]]; then
rm -f "$CORE_FILE" >/dev/null 2>&1
colorize cyan "Existing tunnel services may need a restart after the engine update." bold
sleep 1
fi
[[ -x "$CORE_FILE" ]] && return 0
mkdir -p "$config_dir"
local tmp_file
 tmp_file=$(mktemp)
colorize cyan "Downloading Tixo Aether Engine..." bold
if ! curl -fL --retry 3 --connect-timeout 15 -o "$tmp_file" "$source_url"; then
colorize red "Core download failed: $source_url"
rm -f "$tmp_file"
return 1
fi
install -m 0755 "$tmp_file" "$CORE_FILE"
rm -f "$tmp_file"
colorize green "Tixo Aether Engine installed successfully." bold
}
install_jq
download_tixo_engine
declare -A CONFIG
reset_config() {
CONFIG=()
}
prompt_connection_section() {
local mode="$1"  # server or client
section_header "Link Endpoint"
if [[ "$mode" == "server" ]]; then
prompt_with_default "Bind Address" ":8443" CONFIG[bind_addr]
if [[ -n "${CONFIG[bind_addr]}" && "${CONFIG[bind_addr]}" != *:* ]]; then
CONFIG[bind_addr]=":${CONFIG[bind_addr]}"
fi
else
while true; do
echo -ne "[*] IRAN Server Address [IP:Port] or [Domain:Port]: "
read -r CONFIG[remote_addr]
if [[ -z "${CONFIG[remote_addr]}" ]]; then
colorize red "Server address cannot be empty."
continue
fi
if [[ "${CONFIG[remote_addr]}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}:[0-9]{1,5}$ || \
"${CONFIG[remote_addr]}" =~ ^[a-zA-Z0-9.-]+:[0-9]{1,5}$ ]]; then
break
else
colorize red "Invalid format. Use IP:Port or Domain:Port."
fi
done
if [[ "${CONFIG[transport_type]}" == "ws" || "${CONFIG[transport_type]}" == "wss" || "${CONFIG[transport_type]}" == "wsmux" || "${CONFIG[transport_type]}" == "wssmux" || "${CONFIG[transport_type]}" == "xwsmux" ]]; then
echo -ne "[-] Edge IP/Domain (optional, press Enter to skip): "
read -r CONFIG[edge_ip]
fi
CONFIG[dial_timeout]="10"
CONFIG[retry_interval]="3"
fi
echo ""
}
VALID_ALGORITHMS=("aes-256-gcm" "chacha20-poly1305" "aes-128-gcm")
is_valid_algorithm() {
local input="$1"
for alg in "${VALID_ALGORITHMS[@]}"; do
if [[ "$input" == "$alg" ]]; then
return 0
fi
done
return 1
}
prompt_security_section() {
local is_ipx="$1"
wizard_header "5/8" "ENCRYPTION & IDENTITY" "Secure both endpoints with one shared key"
if [[ "$is_ipx" == "true" ]]; then
prompt_boolean "Enable Encryption" "true" CONFIG[enable_encryption]
if [[ "${CONFIG[enable_encryption]}" == "true" ]]; then
echo
colorize magenta "Encryption algorithm"
select_option "Algorithm" "1" CONFIG[algorithm] \
    "aes-256-gcm" \
    "chacha20-poly1305" \
    "aes-128-gcm"
prompt_shared_key
prompt_with_default "KDF Iterations" "100000" CONFIG[kdf_iterations]
fi
else
prompt_with_default "Security Token" "your_token" CONFIG[token]
CONFIG[enable_encryption]="false"
fi
echo ""
}
prompt_transport_section() {
local mode="$1"
local is_ipx="false"
wizard_header "1/8" "TRANSPORT FABRIC" "Choose the carrier used between both endpoints"
local valid_transports=(tcp tcpmux xtcpmux ws wss wsmux wssmux xwsmux anytls tun)
select_option "Transport" "10" CONFIG[transport_type] "${valid_transports[@]}"
if [[ "${CONFIG[transport_type]}" == "tun" ]]; then
    echo
    wizard_header "2/8" "TUN ENCAPSULATION" "Select standard TCP or native IPX packet mode"
    local encapsulations=(tcp ipx)
    select_option "Encapsulation" "2" CONFIG[tun_encapsulation] "${encapsulations[@]}"
fi
echo
[[ "${CONFIG[tun_encapsulation]}" == "ipx" ]] && is_ipx="true"
if [[ "$is_ipx" != "true" ]]; then
    prompt_boolean "Enable TCP_NODELAY" "true" CONFIG[nodelay]
fi
if [[ "$mode" == "server" ]]; then
    if [[ "${CONFIG[transport_type]}" == "tcp" ]]; then
        prompt_boolean "Accept UDP over TCP" "false" CONFIG[accept_udp]
    fi
    if [[ ! "${CONFIG[transport_type]}" =~ ^(tun|ws)$ ]] && [[ "$is_ipx" != "true" ]]; then
        prompt_boolean "Enable Proxy Protocol" "false" CONFIG[proxy_protocol]
    fi
else
    if [[ "${CONFIG[transport_type]}" != "tun" ]]; then
        prompt_with_default "Connection Pool" "8" CONFIG[connection_pool]
    fi
fi
CONFIG[heartbeat_interval]="10"
CONFIG[heartbeat_timeout]="25"
[[ "$is_ipx" != "true" ]] && CONFIG[keepalive_period]="40"
echo ""
}
prompt_mux_section() {
local transport="$1"
if [[ ! "$transport" =~ mux$ ]]; then
return
fi
section_header "Mux Configuration"
prompt_with_default "Mux Version [1 or 2]" "2" CONFIG[mux_version]
prompt_with_default "Mux Concurrency" "8" CONFIG[mux_concurrency]
CONFIG[mux_framesize]="32768"
CONFIG[mux_recievebuffer]="4194304"
CONFIG[mux_streambuffer]="2097152"
echo ""
}
prompt_tun_section() {
local transport="$1"
local mode="$2"
local is_ipx="$3"
[[ "$transport" != "tun" ]] && return
section_header "Virtual Interface"
prompt_with_default "TUN Device Name" "tixo" CONFIG[tun_name]
local default_local default_remote
if [[ "$mode" == "server" ]]; then
default_local="10.10.10.1/24"
default_remote="10.10.10.2/24"
else
default_local="10.10.10.2/24"
default_remote="10.10.10.1/24"
fi
while true; do
prompt_with_default "TUN Local Address (CIDR)" "$default_local" CONFIG[tun_local_addr]
if validate_cidr "${CONFIG[tun_local_addr]}"; then
break
fi
local suggested=$(validate_cidr "${CONFIG[tun_local_addr]}" 2>&1)
colorize red "Invalid CIDR. Network address should be: $suggested"
done
while true; do
prompt_with_default "TUN Remote Address (CIDR)" "$default_remote" CONFIG[tun_remote_addr]
if validate_cidr "${CONFIG[tun_remote_addr]}"; then
break
fi
colorize red "Invalid CIDR format."
done
prompt_with_default "Health Port" "101" CONFIG[tun_health_port]
if [[ "$is_ipx" == "true" ]]; then
prompt_with_default "MTU" "1320" CONFIG[tun_mtu]
else
prompt_with_default "MTU" "1500" CONFIG[tun_mtu]
fi
echo ""
}
prompt_tls_section() {
local mode="$1"
local transport="$2"
if [[ ! "$transport" =~ ^(anytls|wss|wssmux)$ ]]; then
return
fi
section_header "TLS Configuration"
if [[ "$transport" == "anytls" ]]; then
prompt_with_default "SNI" "www.digikala.com" CONFIG[tls_sni]
fi
if [[ "$mode" == "client" ]]; then
echo
return
fi
if [[ ! -f "$CERT_FILE" || ! -f "$KEY_FILE" ]]; then
colorize red "[*] TLS certificate or key missing, generating self-signed Ed25519 cert..."
openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -x509 -days 365 -sha256 -keyout "$KEY_FILE" -out  "$CERT_FILE" -subj "/CN=tixocloud.com"
colorize green "[*] Generated $CERT_FILE and $KEY_FILE"
echo
fi
prompt_with_default "TLS Certificate Path" "$CERT_FILE" CONFIG[tls_cert]
prompt_with_default "TLS Key Path" "$KEY_FILE" CONFIG[tls_key]
echo ""
}
prompt_tuning_section() {
local is_ipx="$1"
local is_tun="$2"
wizard_header "6/8" "PERFORMANCE PROFILE" "Tune kernel buffers and worker behavior"
prompt_boolean "Enable Auto Tuning" "true" CONFIG[auto_tuning]
echo
colorize magenta "Kernel tuning profile"
select_option "Profile" "1" CONFIG[tuning_profile] \
    "balanced" \
    "fast" \
    "latency" \
    "resource"
prompt_with_default "Workers (0 = auto)" "0" CONFIG[workers]
if [[ "$is_tun" != "true" ]]; then
prompt_with_default "Channel Size" "4096" CONFIG[channel_size]
fi
if [[ "$is_tun" == "true" ]]; then
CONFIG[channel_size]="10_000"
fi
if [[ "$is_ipx" == "true" ]]; then
prompt_with_default "Batch Size" "2048" CONFIG[batch_size]
prompt_with_default "SO_SNDBUF (0 = auto)" "0" CONFIG[so_sndbuf]
else
prompt_with_default "TCP MSS (0 = auto)" "0" CONFIG[tcp_mss]
prompt_with_default "SO_RCVBUF (0 = auto)" "0" CONFIG[so_rcvbuf]
prompt_with_default "SO_SNDBUF (0 = auto)" "0" CONFIG[so_sndbuf]
fi
if [[ "$is_tun" != "true" ]] && [[ "$is_ipx" != "true" ]]; then
echo
colorize magenta "Buffer strategy"
select_option "Buffer profile" "4" CONFIG[buffer_profile] \
    "extreme_low_cpu" \
    "ultra_low_cpu" \
    "low_cpu" \
    "balanced" \
    "low_memory"
prompt_with_default "Read Timeout" "120" CONFIG[read_timeout]
fi
echo ""
}
prompt_logging_section() {
wizard_header "7/8" "TELEMETRY" "Choose the amount of runtime detail"
colorize magenta "Runtime log detail"
select_option "Log level" "5" CONFIG[log_level] \
    "panic" \
    "fatal" \
    "error" \
    "warn" \
    "info" \
    "debug" \
    "trace"
echo ""
}
prompt_accept_udp_section() {
local accept_udp="$1"
[[ "$accept_udp" != "true" ]] && return
CONFIG[ring_size]="64"
CONFIG[frame_size]="2048"
CONFIG[peer_idle_timeout_s]="120"
CONFIG[write_timeout_ms]="3"
}
prompt_ports_section() {
local mode="$1"
local is_tun="$2"
[[ "$mode" != "server" ]] && return
if [[ "$is_tun" != "true" ]]; then
wizard_header "8/8" "ROUTE MAPPING" "Publish one or more services through this tunnel"
printf "  \033[38;5;51m%-18s\033[0m %s\n" "443" "same port on both sides"
printf "  \033[38;5;51m%-18s\033[0m %s\n" "443=5000" "public 443 → destination 5000"
printf "  \033[38;5;51m%-18s\033[0m %s\n" "443-600" "publish an entire port range"
printf "  \033[38;5;51m%-18s\033[0m %s\n" "443-600:5201" "port range → destination 5201"
echo
colorize yellow "Multiple mappings can be separated with commas."
echo -ne "[-] Port map: "
read -r CONFIG[ports_mapping]
echo ""
else
wizard_header "8/8" "ROUTE MAPPING" "Map public ports to destination services"
colorize magenta "Choose the forwarding engine:"
echo "  1) Tixo TCP Relay       — optimized TCP forwarding"
echo "  2) Netfilter Gateway    — TCP + UDP forwarding"
while true; do
    read -r -p "Forwarding engine [1-2] (default: 1): " forwarder_choice
    forwarder_choice="${forwarder_choice:-1}"
    case "$forwarder_choice" in
        1) CONFIG[forwarder]="tixo"; CONFIG[forwarder_label]="Tixo TCP Relay"; break ;;
        2) CONFIG[forwarder]="iptables"; CONFIG[forwarder_label]="Netfilter Gateway"; break ;;
        *) colorize red "Invalid choice. Enter 1 or 2." ;;
    esac
done
echo ""
colorize magenta "Port mapping syntax"
printf "  \033[38;5;51m%-18s\033[0m %s\n" "443" "same port on both sides"
printf "  \033[38;5;51m%-18s\033[0m %s\n" "443=5000" "public 443 → destination 5000"
echo
colorize yellow "Multiple mappings can be separated with commas."
echo -ne "[-] Port map: "
read -r CONFIG[ports_mapping]
echo ""
fi
}
prompt_ipx_section() {
local is_ipx="$1"
local mode="$2"
[[ "$is_ipx" != "true" ]] && return
wizard_header "3/8" "PACKET FABRIC" "Build the IPX path and packet profile"
CONFIG[ipx_mode]="$mode"
AVAILABLE_PROFILES=("icmp" "ipip" "udp" "tcp" "gre" "bip")
select_option "IPX Profile" "4" CONFIG[ipx_profile] "${AVAILABLE_PROFILES[@]}"
prompt_with_default "Listen IP" $SERVER_IP CONFIG[ipx_listen_ip]
while :; do
prompt_with_default "Destination IPv4" "" CONFIG[ipx_dst_ip]
if valid_ipv4 "${CONFIG[ipx_dst_ip]}" && [[ "${CONFIG[ipx_dst_ip]}" != "0.0.0.0" ]]; then
break
fi
colorize red "Invalid destination IPv4. Example: 203.0.113.10"
done
interface=$(ip route show default | awk '{print $5}')
prompt_with_default "Network Interface" "$interface" CONFIG[ipx_interface]

echo ""
wizard_header "4/8" "IP SPOOFING" "Optional custom packet identity for advanced routes"
prompt_boolean "Enable Custom Packet" "false" CONFIG[custom_packet]
if [[ "${CONFIG[custom_packet]}" == "true" ]]; then
    # The input order is identical on IRAN (server) and KHAREJ (client).
    # On KHAREJ, the two entered identities are crossed only while writing TOML.
    while :; do
        prompt_with_default "SRC Spoof IP" "" CONFIG[spoof_src_ip]
        if valid_ipv4 "${CONFIG[spoof_src_ip]}" && [[ "${CONFIG[spoof_src_ip]}" != "0.0.0.0" ]]; then
            break
        fi
        colorize red "Invalid SRC spoof IPv4. Example: 185.143.234.120"
    done
    while :; do
        prompt_with_default "DST Spoof IP" "" CONFIG[spoof_dst_ip]
        if valid_ipv4 "${CONFIG[spoof_dst_ip]}" && [[ "${CONFIG[spoof_dst_ip]}" != "0.0.0.0" ]]; then
            break
        fi
        colorize red "Invalid DST spoof IPv4. Example: 185.143.234.122"
    done
fi

if [[ "${CONFIG[ipx_profile]}" == "icmp" ]]; then
prompt_with_default "ICMP Type" "0" CONFIG[ipx_icmp_type]
prompt_with_default "ICMP Code" "0" CONFIG[ipx_icmp_code]
fi
echo ""
}
generate_toml_config() {
local mode="$1"
local output_file="$2"
local is_tun="$3"
local is_ipx="$4"
{
if [[ "$mode" == "server" ]] && [[ "$is_ipx" == "false" ]]; then
echo "[listener]"
echo "bind_addr = \"${CONFIG[bind_addr]}\""
echo ""
elif [[ "$is_ipx" == "false" ]]; then
echo "[dialer]"
echo "remote_addr = \"${CONFIG[remote_addr]}\""
[[ -n "${CONFIG[edge_ip]}" ]] && echo "edge_ip = \"${CONFIG[edge_ip]}\""
echo "dial_timeout = ${CONFIG[dial_timeout]}"
echo "retry_interval = ${CONFIG[retry_interval]}"
echo ""
fi
echo "[transport]"
echo "type = \"${CONFIG[transport_type]}\""
[[ -n "${CONFIG[nodelay]}" ]] && echo "nodelay = ${CONFIG[nodelay]}"
[[ -n "${CONFIG[keepalive_period]}" ]] && echo "keepalive_period = ${CONFIG[keepalive_period]}"
if [[ "$mode" == "server" ]]; then
[[ -n "${CONFIG[accept_udp]}" ]] && echo "accept_udp = ${CONFIG[accept_udp]}"
[[ -n "${CONFIG[proxy_protocol]}" ]] && echo "proxy_protocol = ${CONFIG[proxy_protocol]}"
else
[[ -n "${CONFIG[connection_pool]}" ]] && [[ "${CONFIG[connection_pool]}" != "0" ]] && \
echo "connection_pool = ${CONFIG[connection_pool]}"
fi
[[ -n "${CONFIG[heartbeat_interval]}" ]] && echo "heartbeat_interval = ${CONFIG[heartbeat_interval]}"
[[ -n "${CONFIG[heartbeat_timeout]}" ]] && echo "heartbeat_timeout = ${CONFIG[heartbeat_timeout]}"
echo ""
if [[ "$is_tun" == "true" ]]; then
echo "[tun]"
echo "encapsulation = \"${CONFIG[tun_encapsulation]}\""
echo "name = \"${CONFIG[tun_name]}\""
echo "local_addr = \"${CONFIG[tun_local_addr]}\""
echo "remote_addr = \"${CONFIG[tun_remote_addr]}\""
echo "health_port = ${CONFIG[tun_health_port]}"
echo "mtu = ${CONFIG[tun_mtu]}"
echo ""
fi
if [[ "$is_ipx" == "true" ]]; then
echo "[ipx]"
echo "mode = \"${CONFIG[ipx_mode]}\""
echo "profile = \"${CONFIG[ipx_profile]}\""
echo "listen_ip = \"${CONFIG[ipx_listen_ip]}\""
echo "dst_ip = \"${CONFIG[ipx_dst_ip]}\""
echo "interface = \"${CONFIG[ipx_interface]}\""
if [[ "${CONFIG[custom_packet]}" == "true" ]]; then
    if [[ "$mode" == "server" ]]; then
        echo "spoof_src_ip = \"${CONFIG[spoof_src_ip]}\""
        echo "spoof_dst_ip = \"${CONFIG[spoof_dst_ip]}\""
    else
        echo "spoof_dst_ip = \"${CONFIG[spoof_src_ip]}\""
        echo "spoof_src_ip = \"${CONFIG[spoof_dst_ip]}\""
    fi
    echo "custom_packet = true"
fi
[[ -n "${CONFIG[ipx_icmp_type]}" ]] && echo "icmp_type = ${CONFIG[ipx_icmp_type]}"
[[ -n "${CONFIG[ipx_icmp_code]}" ]] && echo "icmp_code = ${CONFIG[ipx_icmp_code]}"
echo ""
fi
if [[ "${CONFIG[transport_type]}" =~ mux$ ]]; then
echo "[mux]"
echo "mux_version = ${CONFIG[mux_version]}"
echo "mux_framesize = ${CONFIG[mux_framesize]}"
echo "mux_recievebuffer = ${CONFIG[mux_recievebuffer]}"
echo "mux_streambuffer = ${CONFIG[mux_streambuffer]}"
[[ -n "${CONFIG[mux_concurrency]}" ]] && echo "mux_concurrency = ${CONFIG[mux_concurrency]}"
echo ""
fi
echo "[security]"
if [[ "$is_ipx" == "true" ]]; then
echo "enable_encryption = ${CONFIG[enable_encryption]}"
[[ "${CONFIG[enable_encryption]}" == "true" ]] && {
echo "algorithm = \"${CONFIG[algorithm]}\""
echo "psk = \"${CONFIG[psk]}\""
echo "kdf_iterations = ${CONFIG[kdf_iterations]}"
}
else
echo "token = \"${CONFIG[token]}\""
fi
echo ""
if [[ -n "${CONFIG[tls_sni]}" || -n "${CONFIG[tls_cert]}" ]]; then
echo "[tls]"
[[ -n "${CONFIG[tls_sni]}" ]]  && echo "sni = \"${CONFIG[tls_sni]}\""
[[ -n "${CONFIG[tls_cert]}" ]] && echo "tls_cert = \"${CONFIG[tls_cert]}\""
[[ -n "${CONFIG[tls_key]}" ]]  && echo "tls_key = \"${CONFIG[tls_key]}\""
echo ""
fi
echo "[tuning]"
[[ -n "${CONFIG[auto_tuning]}" ]]     && echo "auto_tuning = ${CONFIG[auto_tuning]}"
[[ -n "${CONFIG[tuning_profile]}" ]]  && echo "tuning_profile = \"${CONFIG[tuning_profile]}\""
[[ -n "${CONFIG[workers]}" ]]         && echo "workers = ${CONFIG[workers]}"
[[ -n "${CONFIG[channel_size]}" ]]    && echo "channel_size = ${CONFIG[channel_size]}"
[[ -n "${CONFIG[tcp_mss]}" ]]         && echo "tcp_mss = ${CONFIG[tcp_mss]}"
[[ -n "${CONFIG[so_rcvbuf]}" ]]       && echo "so_rcvbuf = ${CONFIG[so_rcvbuf]}"
[[ -n "${CONFIG[so_sndbuf]}" ]]       && echo "so_sndbuf = ${CONFIG[so_sndbuf]}"
[[ -n "${CONFIG[buffer_profile]}" ]]  && echo "buffer_profile = \"${CONFIG[buffer_profile]}\""
[[ -n "${CONFIG[batch_size]}" ]]      && echo "batch_size = ${CONFIG[batch_size]}"
[[ -n "${CONFIG[read_timeout]}" ]]    && echo "read_timeout = ${CONFIG[read_timeout]}"
echo ""
if [[ "${CONFIG[accept_udp]}" == "true" ]]; then
echo "[accept_udp]"
echo "ring_size = ${CONFIG[ring_size]}"
echo "frame_size = ${CONFIG[frame_size]}"
echo "peer_idle_timeout_s = ${CONFIG[peer_idle_timeout_s]}"
echo "write_timeout_ms = ${CONFIG[write_timeout_ms]}"
echo ""
fi
echo "[logging]"
echo "log_level = \"${CONFIG[log_level]}\""
echo ""
if [[ "$mode" == "server" ]] ; then
echo "[ports]"
[[ -n "${CONFIG[forwarder]}" ]]  && echo "forwarder = \"${CONFIG[forwarder]}\""
echo "mapping = ["
IFS=',' read -r -a ports <<< "${CONFIG[ports_mapping]}"
for port in "${ports[@]}"; do
[[ -n "$port" ]] && echo "    \"${port// /}\","
done
echo "]"
fi
} > "$output_file"
}
configure_server() {
local mode="$1"  # server or client
local mode_name
if [[ "$mode" == "server" ]]; then
mode_name="IRAN (Server)"
else
mode_name="KHAREJ (Client)"
fi
clear
colorize cyan "Configuring $mode_name" bold
echo ""
reset_config
prompt_transport_section "$mode"
local is_tun="false"
local is_ipx="false"
[[ "${CONFIG[transport_type]}" == "tun" ]] && is_tun="true"
[[ "${CONFIG[tun_encapsulation]}" == "ipx" ]] && is_ipx="true"
prompt_tun_section "${CONFIG[transport_type]}" "$mode" "$is_ipx"
prompt_ipx_section "$is_ipx" "$mode"
if [[ "$is_ipx" != "true" ]]; then
prompt_connection_section "$mode"
fi
prompt_security_section "$is_ipx"
prompt_accept_udp_section "${CONFIG[accept_udp]}"
prompt_mux_section "${CONFIG[transport_type]}"
prompt_tls_section "$mode" "${CONFIG[transport_type]}"
prompt_tuning_section "$is_ipx" "$is_tun"
prompt_logging_section
prompt_ports_section "$mode" "$is_tun"
local tunnel_port
if [[ "$mode" == "server" ]]; then
tunnel_port=$(echo "${CONFIG[bind_addr]}" | grep -oP ':\K[0-9]+$')
else
tunnel_port=$(echo "${CONFIG[remote_addr]}" | grep -oP ':\K[0-9]+$')
fi
if [[ -z "$tunnel_port" ]]; then
tunnel_port=$(echo "${CONFIG[tun_health_port]}")
fi
section_header "Review"
printf "  Role        : %s\n" "$mode_name"
printf "  Transport   : %s\n" "${CONFIG[transport_type]}"
[[ "$is_tun" == "true" ]] && printf "  Interface   : %s (%s)\n" "${CONFIG[tun_name]}" "${CONFIG[tun_encapsulation]}"
if [[ "$is_ipx" == "true" ]]; then
    printf "  IPX Profile : %s\n" "${CONFIG[ipx_profile]}"
    printf "  Destination : %s\n" "${CONFIG[ipx_dst_ip]}"
    printf "  IP Spoofing : %s\n" "${CONFIG[custom_packet]}"
    if [[ "${CONFIG[custom_packet]}" == "true" ]]; then
        printf "  Spoof SRC   : %s\n" "${CONFIG[spoof_src_ip]}"
        printf "  Spoof DST   : %s\n" "${CONFIG[spoof_dst_ip]}"
    fi
fi
if [[ "$mode" == "server" && -n "${CONFIG[forwarder]}" ]]; then
    printf "  Forwarder   : %s\n" "${CONFIG[forwarder_label]:-${CONFIG[forwarder]}}"
    printf "  Core Value  : %s\n" "${CONFIG[forwarder]}"
fi
printf "  Encryption  : %s\n" "${CONFIG[enable_encryption]:-token}"
if [[ "${CONFIG[enable_encryption]}" == "true" && -n "${CONFIG[psk]}" ]]; then
    echo
    printf "  Shared Key  : \033[1;33m%s\033[0m\n" "${CONFIG[psk]}"
    printf "  \033[38;5;245mCopy this key and use the exact same value on the peer server.\033[0m\n"
fi
echo
read -r -p "Create this tunnel? [Y/n]: " confirm_create
confirm_create="${confirm_create:-Y}"
[[ "$confirm_create" =~ ^[Yy]$ ]] || { colorize yellow "Configuration cancelled."; press_key; return 0; }

local config_file service_type service_name temp_config
if [[ "$mode" == "server" ]]; then
config_file="${config_dir}/iran${tunnel_port}.toml"
service_type="iran"
else
config_file="${config_dir}/kharej${tunnel_port}.toml"
service_type="kharej"
fi
service_name="tixotunnel-${service_type}${tunnel_port}.service"

# Never silently overwrite an existing tunnel that uses the same health/listen port.
if [[ -e "$config_file" || -e "${service_dir}/${service_name}" ]]; then
    echo
    colorize red "A tunnel already exists for ${service_type}${tunnel_port}." bold
    printf "  Config  : %s\n" "$config_file"
    printf "  Service : %s\n" "$service_name"
    echo
    read -r -p "Type REPLACE to overwrite this exact tunnel: " replace_confirm
    [[ "$replace_confirm" == "REPLACE" ]] || { colorize yellow "No files were changed."; press_key; return 0; }
    systemctl stop "$service_name" >/dev/null 2>&1 || true
    cp -a "$config_file" "${config_file}.backup-$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
fi

temp_config=$(mktemp "${config_file}.tmp.XXXXXX")
generate_toml_config "$mode" "$temp_config" "$is_tun" "$is_ipx"

# Verify that the generated file contains the values entered in this wizard.
if [[ "$is_ipx" == "true" ]]; then
    grep -Fqx "dst_ip = \"${CONFIG[ipx_dst_ip]}\"" "$temp_config" || { rm -f "$temp_config"; colorize red "Config verification failed: destination mismatch."; press_key; return 1; }
    if [[ "${CONFIG[custom_packet]}" == "true" ]]; then
        local expected_spoof_src expected_spoof_dst
        if [[ "$mode" == "server" ]]; then
            expected_spoof_src="${CONFIG[spoof_src_ip]}"
            expected_spoof_dst="${CONFIG[spoof_dst_ip]}"
        else
            expected_spoof_src="${CONFIG[spoof_dst_ip]}"
            expected_spoof_dst="${CONFIG[spoof_src_ip]}"
        fi
        grep -Fqx "spoof_src_ip = \"${expected_spoof_src}\"" "$temp_config" || { rm -f "$temp_config"; colorize red "Config verification failed: spoof SRC mismatch."; press_key; return 1; }
        grep -Fqx "spoof_dst_ip = \"${expected_spoof_dst}\"" "$temp_config" || { rm -f "$temp_config"; colorize red "Config verification failed: spoof DST mismatch."; press_key; return 1; }
        grep -Fqx "custom_packet = true" "$temp_config" || { rm -f "$temp_config"; colorize red "Config verification failed: custom packet flag missing."; press_key; return 1; }
    fi
fi
if [[ "$mode" == "server" && -n "${CONFIG[forwarder]}" ]]; then
    grep -Fqx "forwarder = \"${CONFIG[forwarder]}\"" "$temp_config" || { rm -f "$temp_config"; colorize red "Config verification failed: forwarder mismatch."; press_key; return 1; }
fi

mv -f "$temp_config" "$config_file"
chmod 600 "$config_file"
create_systemd_service "$service_type" "$tunnel_port" "$config_file"

echo
if ! systemctl is-active --quiet "$service_name"; then
    colorize red "Service creation failed or the engine exited." bold
    printf "  Config  : %s\n" "$config_file"
    printf "  Service : %s\n\n" "$service_name"
    journalctl -u "$service_name" -n 25 --no-pager -o cat 2>/dev/null | brand_engine_output
    press_key
    return 1
fi

colorize green "Configuration completed and verified." bold
printf "\n  Config file : %s\n" "$config_file"
printf "  Service     : %s\n" "$service_name"
printf "  ExecStart   : %s -c %s\n" "$CORE_FILE" "$config_file"
if [[ "$is_ipx" == "true" ]]; then
    printf "  Destination : %s\n" "${CONFIG[ipx_dst_ip]}"
    if [[ "${CONFIG[custom_packet]}" == "true" ]]; then
        printf "  Spoof SRC   : %s\n" "${CONFIG[spoof_src_ip]}"
        printf "  Spoof DST   : %s\n" "${CONFIG[spoof_dst_ip]}"
    fi
fi
if [[ "$mode" == "server" && -n "${CONFIG[forwarder]}" ]]; then
    printf "  Forwarder   : %s (core value: %s)\n" "${CONFIG[forwarder_label]:-${CONFIG[forwarder]}}" "${CONFIG[forwarder]}"
fi
echo
colorize yellow "Note: a connected health service confirms only the control/health channel, not forwarded traffic."
echo
press_key
}
create_systemd_service() {
local type="$1"
local port="$2"
local config_file="$3"
local service_file="${service_dir}/tixotunnel-${type}${port}.service"
local desc_type="$(tr '[:lower:]' '[:upper:]' <<< "${type:0:1}")${type:1}"
cat > "$service_file" <<EOF
[Unit]
Description=TixoTunnel $desc_type Port $port
After=network.target
[Service]
Type=simple
User=root
ExecStart=${CORE_FILE} -c $config_file
Restart=always
RestartSec=3
LimitNOFILE=1048576
TasksMax=infinity
LimitMEMLOCK=infinity
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now "tixotunnel-${type}${port}.service" >/dev/null 2>&1
colorize green "✔ Service tixotunnel-${type}${port} created and started" bold
}
SERVER_IP=$(hostname -I | awk '{print $1}')
SERVER_COUNTRY=$(curl -sS --max-time 1 "http://ipwhois.app/json/$SERVER_IP" 2>/dev/null | jq -r '.country')
SERVER_ISP=$(curl -sS --max-time 1 "http://ipwhois.app/json/$SERVER_IP" 2>/dev/null | jq -r '.isp')
display_logo() {
clear
local red="\033[38;5;196m" white="\033[97m" gray="\033[38;5;245m"
local cyan="\033[38;5;51m" green="\033[38;5;46m" reset="\033[0m"
echo -e "${red}"
echo "___________.__             _________ .__                   .___"
echo "\\__    ___/|__|__  _______ \\_   ___ \\|  |   ____  __ __  __| _/"
echo "  |    |   |  \\  \\/  /  _ \\/    \\  \\/|  |  /  _ \\|  |  \\/ __ |"
echo "  |    |   |  |>    <  <_> )     \\___|  |_(  <_> )  |  / /_/ |"
echo "  |____|   |__/__/\\_ \\____/ \\______  /____/\\____/|____/\\____ |"
echo "                    \\/             \\/                       \\/"
echo -e "${reset}"
echo -e "${gray}════════════════════════════════════════════════════════════════════${reset}"
printf "${white} %-10s${reset} ${cyan}%-15s${reset} ${white}%-10s${reset} ${green}%s${reset}\n" \
    "Console" "$SCRIPT_VERSION" "Engine" "$ENGINE_EDITION"
printf "${white} %-10s${reset} ${green}%-15s${reset} ${white}%-10s${reset} ${cyan}%s${reset}\n" \
    "Status" "● Operational" "Channel" "$BRAND_CHANNEL"
printf "${white} %-10s${reset} ${gray}%s${reset}\n" "Features" "TUN • IPX • Encryption • Auto Tuning • IP Spoofing"
echo -e "${gray}════════════════════════════════════════════════════════════════════${reset}"
}

get_active_tunnel_count() {
local count=0 service
for service in "$service_dir"/tixotunnel-*.service; do
    [[ -e "$service" ]] || continue
    systemctl is-active --quiet "$(basename "$service")" && ((count++))
done
echo "$count"
}
get_total_tunnel_count() {
find "$config_dir" -maxdepth 1 -type f \( -name 'iran*.toml' -o -name 'kharej*.toml' \) 2>/dev/null | wc -l | tr -d ' '
}
get_cpu_usage() {
local a b idle total prev_idle prev_total diff_idle diff_total
read -r _ a b _ _ idle _ _ _ _ _ < /proc/stat
prev_idle=$idle; prev_total=$((a+b+idle)); sleep 0.12
read -r _ a b _ _ idle _ _ _ _ _ < /proc/stat
idle=$idle; total=$((a+b+idle)); diff_idle=$((idle-prev_idle)); diff_total=$((total-prev_total))
(( diff_total > 0 )) && echo $((100-(100*diff_idle/diff_total))) || echo 0
}
display_server_info() {
local gray="\033[38;5;245m" cyan="\033[38;5;51m" green="\033[38;5;46m" reset="\033[0m"
local active total cpu ram disk uptime_short
active=$(get_active_tunnel_count); total=$(get_total_tunnel_count)
cpu=$(get_cpu_usage)
ram=$(free | awk '/Mem:/ {printf "%.0f", $3/$2*100}')
disk=$(df -P / | awk 'NR==2 {gsub(/%/,"",$5); print $5}')
uptime_short=$(uptime -p 2>/dev/null | sed 's/^up //' || true)
[[ -z "$SERVER_COUNTRY" || "$SERVER_COUNTRY" == "null" ]] && SERVER_COUNTRY="Unknown"
[[ -z "$SERVER_ISP" || "$SERVER_ISP" == "null" ]] && SERVER_ISP="Unknown"
echo -e "${cyan} SERVER${reset}"
printf "  %-12s : %s\n" "IP Address" "$SERVER_IP"
printf "  %-12s : %s\n" "Location" "$SERVER_COUNTRY"
printf "  %-12s : %s\n" "Provider" "$SERVER_ISP"
printf "  %-12s : %s active / %s total\n" "Tunnels" "$active" "$total"
echo -e "${gray}────────────────────────────────────────────────────────────────────${reset}"
echo -e "${cyan} SYSTEM${reset}"
printf "  CPU %-4s%%    RAM %-4s%%    DISK %-4s%%    UPTIME %s\n" "$cpu" "$ram" "$disk" "${uptime_short:-Unknown}"
echo -e "${gray}════════════════════════════════════════════════════════════════════${reset}"
}
display_engine_status() {
if [[ -x "$CORE_FILE" ]]; then
    echo -e "\033[38;5;46m ● Engine ready\033[0m  \033[38;5;245mTUN • IPX • Spoof Support\033[0m"
else
    echo -e "\033[38;5;196m ● Engine missing\033[0m"
fi
}

check_config_backup() {
missing_services=()
for config in "${config_dir}"/iran*.toml "${config_dir}"/kharej*.toml; do
[ -e "$config" ] || continue
fname=$(basename "$config")
if [[ "$fname" =~ ^(iran|kharej)([0-9]+)\.toml$ ]]; then
location="${BASH_REMATCH[1]}"
tunnel_port="${BASH_REMATCH[2]}"
service_file="${service_dir}/tixotunnel-${location}${tunnel_port}.service"
if [[ ! -f "$service_file" ]]; then
missing_services+=("$service_file:$location:$tunnel_port")
fi
fi
done
[[ ${#missing_services[@]} -eq 0 ]] && return 0
echo
colorize red "Missing service files:" bold
for entry in "${missing_services[@]}"; do
service_file="${entry%%:*}"
location="${entry#*:}"; location="${location%%:*}"
tunnel_port="${entry##*:}"
echo "- $service_file (type: $location, port: $tunnel_port)"
done
echo
read -r -p "Do you want to create missing service files? (y/n): " confirm
if [[ "$confirm" =~ ^[Yy]$ ]]; then
for entry in "${missing_services[@]}"; do
service_file="${entry%%:*}"
location="${entry#*:}"; location="${location%%:*}"
tunnel_port="${entry##*:}"
config_file="${config_dir}/${location}${tunnel_port}.toml"
desc_loc="$(tr '[:lower:]' '[:upper:]' <<< "${location:0:1}")${location:1}"
cat > "$service_file" <<EOF
[Unit]
Description=TixoTunnel $desc_loc Port $tunnel_port
After=network.target
[Service]
Type=simple
User=root
ExecStart=${CORE_FILE} -c $config_file
Restart=always
RestartSec=3
LimitNOFILE=1048576
TasksMax=infinity
LimitMEMLOCK=infinity
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now "$(basename "$service_file")"
echo "Created and started $(basename "$service_file")"
done
fi
sleep 2
}
check_config_backup
check_tunnel_status() {
if ! ls "$config_dir"/*.toml 1> /dev/null 2>&1; then
colorize red "No config files found." bold
press_key
return 1
fi
clear
colorize yellow "Checking TixoTunnel services..." bold
sleep 1
echo
for config_path in "$config_dir"/{iran,kharej}*.toml; do
[ -f "$config_path" ] || continue
config_name=$(basename "$config_path")
config_name="${config_name%.toml}"
service_name="tixotunnel-${config_name}.service"
if [[ "$config_name" =~ ^iran([0-9]+)$ ]]; then
port="${BASH_REMATCH[1]}"
if systemctl is-active --quiet "$service_name"; then
colorize green "Iran service (port $port) is running"
else
colorize red "Iran service (port $port) is not running"
fi
elif [[ "$config_name" =~ ^kharej([0-9]+)$ ]]; then
port="${BASH_REMATCH[1]}"
if systemctl is-active --quiet "$service_name"; then
colorize green "Kharej service (port $port) is running"
else
colorize red "Kharej service (port $port) is not running"
fi
fi
done
echo
press_key
}
scheduler_unit_base() {
local service="${1%.service}"
printf "/etc/systemd/system/%s-auto-restart" "$service"
}
restart_scheduler() {
local service="$1" base timer_file service_file current="Disabled" choice value unit
base=$(scheduler_unit_base "$service")
timer_file="${base}.timer"
service_file="${base}.service"
[[ -f "$timer_file" ]] && current=$(grep -E '^OnUnitActiveSec=' "$timer_file" | cut -d= -f2-)
while true; do
    clear
    section_header "Restart Scheduler"
    printf "  Service     : %s\n" "$service"
    printf "  Schedule    : %s\n\n" "$current"
    echo "  [1] Set interval"
    echo "  [2] Disable scheduler"
    echo "  [0] Back"
    echo
    read -r -p "Select an option [0-2]: " choice
    case "$choice" in
        1)
            echo
            echo "  [1] Minutes"
            echo "  [2] Hours"
            read -r -p "Interval unit [1-2]: " unit
            [[ "$unit" == "1" || "$unit" == "2" ]] || { colorize red "Invalid unit."; sleep 1; continue; }
            read -r -p "Restart every how many $([[ "$unit" == "1" ]] && echo minutes || echo hours)? " value
            [[ "$value" =~ ^[1-9][0-9]*$ ]] || { colorize red "Enter a positive whole number."; sleep 1; continue; }
            [[ "$unit" == "1" ]] && interval="${value}min" || interval="${value}h"
            cat > "$service_file" <<EOF
[Unit]
Description=Scheduled restart for $service

[Service]
Type=oneshot
ExecStart=/bin/systemctl restart $service
EOF
            cat > "$timer_file" <<EOF
[Unit]
Description=Auto restart timer for $service

[Timer]
OnBootSec=$interval
OnUnitActiveSec=$interval
AccuracySec=1min
Persistent=true

[Install]
WantedBy=timers.target
EOF
            systemctl daemon-reload
            systemctl enable --now "$(basename "$timer_file")" >/dev/null 2>&1
            current="$interval"
            colorize green "Scheduler enabled: every $interval" bold
            sleep 2
            ;;
        2)
            systemctl disable --now "$(basename "$timer_file")" >/dev/null 2>&1 || true
            rm -f "$timer_file" "$service_file"
            systemctl daemon-reload
            current="Disabled"
            colorize green "Scheduler disabled." bold
            sleep 2
            ;;
        0) return ;;
        *) colorize red "Invalid option."; sleep 1 ;;
    esac
done
}

tunnel_management() {
if ! ls "$config_dir"/*.toml 1> /dev/null 2>&1; then
colorize red "No config files found." bold
press_key
return 1
fi
clear
colorize cyan "TixoTunnel Manager" bold
echo
local index=1
declare -a configs
for config_path in "$config_dir"/{iran,kharej}*.toml; do
[ -f "$config_path" ] || continue
config_name=$(basename "$config_path")
if [[ "$config_name" =~ ^iran([0-9]+)\.toml$ ]]; then
port="${BASH_REMATCH[1]}"
configs+=("$config_path")
service_name="tixotunnel-iran${port}.service"
if systemctl is-active --quiet "$service_name"; then status="\033[38;5;46m● Running\033[0m"; else status="\033[38;5;245m○ Stopped\033[0m"; fi
echo -e "\033[35m${index}\033[0m) Iran · Port \033[33m$port\033[0m · $status"
((index++))
elif [[ "$config_name" =~ ^kharej([0-9]+)\.toml$ ]]; then
port="${BASH_REMATCH[1]}"
configs+=("$config_path")
service_name="tixotunnel-kharej${port}.service"
if systemctl is-active --quiet "$service_name"; then status="\033[38;5;46m● Running\033[0m"; else status="\033[38;5;245m○ Stopped\033[0m"; fi
echo -e "\033[35m${index}\033[0m) Kharej · Port \033[33m$port\033[0m · $status"
((index++))
fi
done
echo
echo -ne "Enter your choice (0 to return): "
read -r choice
[[ "$choice" == "0" ]] && return
while ! [[ "$choice" =~ ^[0-9]+$ ]] || (( choice < 1 || choice > ${#configs[@]} )); do
colorize red "Invalid choice."
echo -ne "Enter your choice (0 to return): "
read -r choice
[[ "$choice" == "0" ]] && return
done
selected_config="${configs[$((choice - 1))]}"
config_name=$(basename "${selected_config%.toml}")
service_name="tixotunnel-${config_name}.service"
clear
colorize cyan "Tunnel Control — $config_name" bold
echo
colorize red "1) Remove Tunnel"
colorize yellow "2) Restart Tunnel"
echo "3) Live Monitor"
echo "4) Service Details"
echo "5) Restart Scheduler"
echo
read -r -p "Enter your choice (0 to return): " choice
case $choice in
1) destroy_tunnel "$selected_config" ;;
2) restart_service "$service_name" ;;
3) view_service_logs "$service_name" ;;
4) view_service_status "$service_name" ;;
5) restart_scheduler "$service_name" ;;
0) return ;;
*) colorize red "Invalid option!" && sleep 1 ;;
esac
}
destroy_tunnel() {
config_path="$1"
config_name=$(basename "${config_path%.toml}")
service_name="tixotunnel-${config_name}.service"
service_path="$service_dir/$service_name"
[ -f "$config_path" ] && rm -f "$config_path"
local scheduler_base
scheduler_base=$(scheduler_unit_base "$service_name")
systemctl disable --now "$(basename "${scheduler_base}.timer")" >/dev/null 2>&1 || true
rm -f "${scheduler_base}.timer" "${scheduler_base}.service"
if [[ -f "$service_path" ]]; then
systemctl is-active --quiet "$service_name" && systemctl disable --now "$service_name" >/dev/null 2>&1
rm -f "$service_path"
fi
systemctl daemon-reload
echo
colorize green "Tunnel destroyed successfully!" bold
echo
press_key
}
restart_service() {
echo
colorize yellow "Restarting $1" bold
if systemctl list-units --type=service | grep -q "$1"; then
systemctl restart "$1"
colorize green "Service restarted successfully" bold
echo
else
colorize red "Service not found"
fi
press_key
}
brand_engine_output() {
sed -u -E \
  -e '/^[[:space:]]*[╔║╚].*[╗║╝][[:space:]]*$/d' \
  -e '/Backhaul v[0-9]/d' \
  -e '/High-Performance Reverse Network Tunnel/d' \
  -e '/^[[:space:]]*root[[:space:]]*:[[:space:]]*PWD=/d' \
  -e '/pam_unix\(sudo:session\)/d' \
  -e '/^[[:space:]]*🌐 IP Address:/d' \
  -e '/^[[:space:]]*📋 Configuration Summary:/d' \
  -e '/^[[:space:]]*🔧 General Settings:/d' \
  -e '/^[[:space:]]*🚀 Starting /d' \
  -e '/^[[:space:]]*Mode: /d' \
  -e '/^[[:space:]]*Log Level: /d' \
  -e '/^[[:space:]]*Transport: /d' \
  -e '/^[[:space:]]*TCP Optimization: /d' \
  -e '/^═+$/d' \
  -e 's/Backhaul/Tixo Aether/g' \
  -e 's/bbackhaul/Tixo TCP Relay/g' \
  -e 's/backhaul/tixo-engine/g' \
  -e 's/Starting Ipx/Starting IPX Fabric/g' \
  -e 's/custom packet status:/IP spoofing:/g' \
  -e 's/[♥❤♡❣💙💚💛💜🖤🤍🤎💔💕💞💓💗💖💘💝]️*//g' \
  -e 's/tixo-fwd/tixo/g'
}
show_live_header() {
local service="$1"
echo -e "\033[38;5;51m════════════════════════════════════════════════════════════════════\033[0m"
echo -e "\033[97m TixoTunnel Live Monitor\033[0m"
printf  "\033[38;5;245m Service  %-36s Engine  %s\033[0m\n" "$service" "$ENGINE_EDITION"
echo -e "\033[38;5;51m════════════════════════════════════════════════════════════════════\033[0m"
}

view_service_logs() {
clear
show_live_header "$1"
colorize yellow "Press Ctrl+C to return"
journalctl -eu "$1" -f -o cat | brand_engine_output
}
view_service_status() {
clear
show_live_header "$1"
systemctl status "$1" --no-pager | brand_engine_output
press_key
}
remove_core() {
if find "$config_dir" -type f -name "*.toml" | grep -q .; then
colorize red "Delete all services first."
sleep 3
return 1
fi
colorize yellow "Remove the Tixo Aether Engine and its files? (y/n)"
read -r confirm
if [[ $confirm == [yY] ]]; then
[[ -d "$config_dir" ]] && rm -rf "$config_dir"
colorize green "Tixo Aether Engine removed successfully." bold
fi
press_key
}
update_script() {
  unified_update
}

create_update_backup() {
  mkdir -p "$UPDATE_BACKUP_DIR"
  local stamp archive manifest
  stamp=$(date +%Y%m%d-%H%M%S)
  archive="$UPDATE_BACKUP_DIR/tixotunnel-update-$stamp.tar.gz"
  manifest=$(mktemp)
  for f in "$PANEL_PATH" "$COMMAND_PATH" "$CORE_FILE" "$SPOOF_TESTER_FILE"; do
    [[ -e "$f" ]] && echo "$f" >> "$manifest"
  done
  find /etc/systemd/system -maxdepth 1 -type f \( -name 'tixotunnel-*.service' -o -name 'tixotunnel-*.timer' \) -print >> "$manifest" 2>/dev/null || true
  tar -czf "$archive" --absolute-names --files-from "$manifest" 2>/dev/null || { rm -f "$manifest" "$archive"; return 1; }
  rm -f "$manifest"
  printf '%s' "$archive"
}

restart_active_tunnels() {
  local unit
  while read -r unit; do
    [[ -n "$unit" ]] || continue
    systemctl restart "$unit" >/dev/null 2>&1 || true
  done < <(systemctl list-units --type=service --state=running 'tixotunnel-*.service' --no-legend 2>/dev/null | awk '{print $1}')
}

unified_update() {
  clear; section_header "Unified Update"
  colorize cyan "This updates the console, AETHER-X1 core, Spoof Tester and the tixotunnel command in one operation." bold
  echo
  read -r -p "Update all TixoTunnel components now? [Y/n]: " answer
  [[ "$answer" =~ ^[Nn]$ ]] && return

  local tmpdir backup panel_tmp core_tmp tester_tmp rc=0
  tmpdir=$(mktemp -d)
  panel_tmp="$tmpdir/TixoTunnel.sh"
  core_tmp="$tmpdir/tixotunnel-core"
  tester_tmp="$tmpdir/tixotunnel-core.engine"
  trap 'rm -rf "$tmpdir"' RETURN

  echo; colorize cyan "[1/8] Creating rollback backup..." bold
  backup=$(create_update_backup) || { colorize red "Backup creation failed. Update cancelled."; press_key; return 1; }
  colorize green "      OK — $backup"

  echo; colorize cyan "[2/8] Downloading console..." bold
  curl -fL --retry 3 --connect-timeout 15 -o "$panel_tmp" "$PANEL_DOWNLOAD_URL" || rc=1
  ((rc==0)) && colorize green "      OK" || { colorize red "      FAILED"; press_key; return 1; }

  echo; colorize cyan "[3/8] Downloading AETHER-X1 core..." bold
  curl -fL --retry 3 --connect-timeout 15 -o "$core_tmp" "$CORE_DOWNLOAD_URL" || rc=1
  ((rc==0)) && colorize green "      OK" || { colorize red "      FAILED"; press_key; return 1; }

  echo; colorize cyan "[4/8] Downloading Spoof Tester..." bold
  if ! curl -fL --retry 3 --connect-timeout 15 -o "$tester_tmp" "$SPOOF_TESTER_DOWNLOAD_URL"; then
    curl -fL --retry 3 --connect-timeout 15 -o "$tester_tmp" "$SPOOF_TESTER_FALLBACK_URL" || rc=1
  fi
  ((rc==0)) && colorize green "      OK" || { colorize red "      FAILED"; press_key; return 1; }

  echo; colorize cyan "[5/8] Verifying downloaded files..." bold
  if [[ ! -s "$panel_tmp" || ! -s "$core_tmp" || ! -s "$tester_tmp" ]]; then
    colorize red "      Verification failed: one or more files are empty."; press_key; return 1
  fi
  bash -n "$panel_tmp" || { colorize red "      Console syntax verification failed."; press_key; return 1; }
  chmod 0755 "$core_tmp" "$tester_tmp"
  colorize green "      OK"

  echo; colorize cyan "[6/8] Installing all components..." bold
  mkdir -p "$config_dir"
  if ! install -m0755 "$panel_tmp" "$PANEL_PATH" || \
     ! install -m0755 "$panel_tmp" "$COMMAND_PATH" || \
     ! install -m0755 "$core_tmp" "$CORE_FILE" || \
     ! install -m0755 "$tester_tmp" "$SPOOF_TESTER_FILE"; then
    colorize red "      Installation failed. Restoring backup..."
    tar -xzf "$backup" -C / >/dev/null 2>&1 || true
    systemctl daemon-reload
    press_key; return 1
  fi
  colorize green "      OK"

  echo; colorize cyan "[7/8] Reloading services..." bold
  systemctl daemon-reload
  restart_active_tunnels
  colorize green "      OK"

  echo; colorize cyan "[8/8] Final validation..." bold
  [[ -x "$CORE_FILE" && -x "$SPOOF_TESTER_FILE" ]] || {
    colorize red "      Validation failed. Restoring backup..."
    tar -xzf "$backup" -C / >/dev/null 2>&1 || true
    systemctl daemon-reload; restart_active_tunnels; press_key; return 1
  }
  colorize green "      OK"
  echo
  colorize green "TixoTunnel and all components were updated successfully." bold
  colorize cyan "Rollback backup: $backup"
  sleep 2
  exec "$PANEL_PATH"
}

rollback_last_update() {
  mkdir -p "$UPDATE_BACKUP_DIR"
  local backup
  backup=$(ls -1t "$UPDATE_BACKUP_DIR"/tixotunnel-update-*.tar.gz 2>/dev/null | head -1)
  [[ -f "$backup" ]] || { colorize yellow "No unified-update backup was found."; press_key; return; }
  clear; section_header "Rollback Last Update"
  printf "  Backup: %s\n\n" "$backup"
  read -r -p "Type ROLLBACK to restore it: " confirm
  [[ "$confirm" == ROLLBACK ]] || return
  tar -xzf "$backup" -C / || { colorize red "Rollback failed."; press_key; return 1; }
  systemctl daemon-reload
  restart_active_tunnels
  colorize green "Rollback completed successfully." bold
  sleep 2
  exec "$PANEL_PATH"
}

configure_tunnel() {
[[ ! -d "$config_dir" ]] && {
colorize red "Install the Tixo Aether Engine first."
press_key
return 1
}
clear
section_header "Create Tunnel"
echo -e "  \033[38;5;51m[1]\033[0m IRAN   \033[38;5;245mServer / Listener\033[0m"
echo -e "  \033[38;5;51m[2]\033[0m KHAREJ \033[38;5;245mClient / Connector\033[0m"
echo -e "  \033[38;5;245m[0] Back\033[0m"
echo
read -r -p "Select endpoint role [0-2]: " configure_choice
case "$configure_choice" in
1) configure_server "server" ;;
2) configure_server "client" ;;
0) return ;;
*) colorize red "Invalid option!" && sleep 1 ;;
esac
}

get_public_ip() { curl -4fsS --connect-timeout 3 --max-time 6 https://api.ipify.org 2>/dev/null || echo "Unavailable"; }

get_geo_information() {
  local public_ip="$1" data="" org=""
  GEO_COUNTRY="Unknown"; GEO_COUNTRY_CODE="Unknown"; GEO_CITY="Unknown"
  GEO_REGION="Unknown"; GEO_TIMEZONE="Unknown"; GEO_ASN="Unknown"
  GEO_ISP="Unknown"; GEO_HOSTNAME="Unknown"

  command -v curl >/dev/null 2>&1 || return 1
  command -v jq >/dev/null 2>&1 || return 1
  [[ -n "$public_ip" && "$public_ip" != "Unavailable" ]] || return 1

  # Provider 1: ipinfo.io. Force IPv4 so the lookup matches Public IPv4.
  data=$(curl -4fsS --connect-timeout 4 --max-time 8     "https://ipinfo.io/${public_ip}/json" 2>/dev/null || true)
  if jq -e '.ip and .country' >/dev/null 2>&1 <<< "$data"; then
    GEO_COUNTRY_CODE=$(jq -r '.country // "Unknown"' <<< "$data")
    GEO_COUNTRY="$GEO_COUNTRY_CODE"
    GEO_CITY=$(jq -r '.city // "Unknown"' <<< "$data")
    GEO_REGION=$(jq -r '.region // "Unknown"' <<< "$data")
    GEO_TIMEZONE=$(jq -r '.timezone // "Unknown"' <<< "$data")
    GEO_HOSTNAME=$(jq -r '.hostname // "Unknown"' <<< "$data")
    org=$(jq -r '.org // empty' <<< "$data")
    if [[ -n "$org" ]]; then
      GEO_ASN=$(awk '{print $1}' <<< "$org")
      GEO_ISP=$(cut -d' ' -f2- <<< "$org")
      [[ -n "$GEO_ISP" ]] || GEO_ISP="Unknown"
    fi
    return 0
  fi

  # Provider 2: ipwho.is. Keep IPv4 forced to avoid an IPv6/IPv4 mismatch.
  data=$(curl -4fsS --connect-timeout 4 --max-time 8     "https://ipwho.is/${public_ip}" 2>/dev/null || true)
  if jq -e '.success == true' >/dev/null 2>&1 <<< "$data"; then
    GEO_COUNTRY=$(jq -r '.country // "Unknown"' <<< "$data")
    GEO_COUNTRY_CODE=$(jq -r '.country_code // "Unknown"' <<< "$data")
    GEO_CITY=$(jq -r '.city // "Unknown"' <<< "$data")
    GEO_REGION=$(jq -r '.region // "Unknown"' <<< "$data")
    GEO_TIMEZONE=$(jq -r '.timezone.id // "Unknown"' <<< "$data")
    GEO_ASN=$(jq -r 'if .connection.asn then "AS" + (.connection.asn|tostring) else "Unknown" end' <<< "$data")
    GEO_ISP=$(jq -r '.connection.isp // .connection.org // "Unknown"' <<< "$data")
    return 0
  fi
  return 1
}

network_information() {
  clear; section_header "Network & System Information"
  local iface gateway ip pub uptime_s load mem disk kernel os arch virt cpu cores threads swap hostname ipv6
  iface=$(ip -4 route show default 2>/dev/null | awk 'NR==1{print $5}')
  gateway=$(ip -4 route show default 2>/dev/null | awk 'NR==1{print $3}')
  ip=$(ip -4 addr show "$iface" 2>/dev/null | awk '/inet /{print $2; exit}')
  ipv6=$(ip -6 addr show scope global 2>/dev/null | awk '/inet6 /{print $2; exit}')
  pub=$(get_public_ip)
  get_geo_information "$pub" || true
  hostname=$(hostname 2>/dev/null || echo Unknown)
  os=$(awk -F= '/^PRETTY_NAME=/{gsub(/^"|"$/,"",$2); print $2}' /etc/os-release 2>/dev/null)
  kernel=$(uname -r); arch=$(uname -m)
  virt=$(systemd-detect-virt 2>/dev/null || echo Unknown)
  cpu=$(awk -F: '/model name/{gsub(/^[ \t]+/,"",$2); print $2; exit}' /proc/cpuinfo)
  cores=$(nproc 2>/dev/null || echo Unknown)
  threads=$(grep -c '^processor' /proc/cpuinfo 2>/dev/null || echo Unknown)
  uptime_s=$(uptime -p 2>/dev/null | sed 's/^up //')
  load=$(awk '{print $1", "$2", "$3}' /proc/loadavg)
  mem=$(free -h | awk '/Mem:/{print $3" / "$2" ("int($3*100/$2)"%)"}')
  swap=$(free -h | awk '/Swap:/{if($2=="0B"||$2=="0") print "Disabled"; else print $3" / "$2}')
  disk=$(df -h / | awk 'NR==2{print $3" / "$2" ("$5")"}')

  printf "  %-17s : %s\n" "Hostname" "${hostname:-Unknown}"
  printf "  %-17s : %s\n" "OS" "${os:-Unknown}"
  printf "  %-17s : %s\n" "Kernel" "$kernel"
  printf "  %-17s : %s\n" "Architecture" "$arch"
  printf "  %-17s : %s\n" "Virtualization" "${virt:-Unknown}"
  echo
  printf "  %-17s : %s\n" "Public IPv4" "$pub"
  printf "  %-17s : %s\n" "Public IPv6" "${ipv6:-Unavailable}"
  printf "  %-17s : %s\n" "Interface" "${iface:-Unknown}"
  printf "  %-17s : %s\n" "Interface IPv4" "${ip:-Unknown}"
  printf "  %-17s : %s\n" "Gateway" "${gateway:-Unknown}"
  printf "  %-17s : %s\n" "Reverse DNS" "${GEO_HOSTNAME:-Unknown}"
  echo
  printf "  %-17s : %s\n" "Country" "${GEO_COUNTRY:-Unknown} (${GEO_COUNTRY_CODE:-Unknown})"
  printf "  %-17s : %s\n" "City / Region" "${GEO_CITY:-Unknown} / ${GEO_REGION:-Unknown}"
  printf "  %-17s : %s\n" "Timezone" "${GEO_TIMEZONE:-Unknown}"
  printf "  %-17s : %s\n" "ASN" "${GEO_ASN:-Unknown}"
  printf "  %-17s : %s\n" "ISP" "${GEO_ISP:-Unknown}"
  echo
  printf "  %-17s : %s\n" "CPU" "${cpu:-Unknown}"
  printf "  %-17s : %s\n" "CPU Threads" "${threads:-$cores}"
  printf "  %-17s : %s\n" "Load Average" "$load"
  printf "  %-17s : %s\n" "Memory" "$mem"
  printf "  %-17s : %s\n" "Swap" "$swap"
  printf "  %-17s : %s\n" "Disk" "$disk"
  printf "  %-17s : %s\n" "Uptime" "$uptime_s"
  echo; press_key
}

check_updates() {
  clear; section_header "Update Checker"
  local remote tmp
  tmp=$(mktemp)
  if curl -fsSL --max-time 8 "https://raw.githubusercontent.com/${GITHUB_REPO}/main/TixoTunnel.sh" -o "$tmp"; then
    remote=$(grep -m1 '^SCRIPT_VERSION=' "$tmp" | cut -d'"' -f2)
    printf "  Installed : %s\n  Available : %s\n\n" "$SCRIPT_VERSION" "${remote:-Unknown}"
    if [[ "$remote" == "$SCRIPT_VERSION" ]]; then colorize green "You are running the latest console." bold
    elif [[ -n "$remote" ]]; then colorize yellow "A newer console is available." bold
    else colorize red "Could not read remote version."; fi
  else colorize red "Update server is unreachable."; fi
  rm -f "$tmp"; echo; press_key
}

connection_benchmark() {
  clear; section_header "Connection Benchmark"
  local target count rtt loss mtu tcp_port
  read -r -p "Target IP or hostname: " target
  [[ -n "$target" ]] || return
  read -r -p "TCP port [443]: " tcp_port; tcp_port=${tcp_port:-443}
  echo; colorize cyan "Running network diagnostics..." bold
  local pingout; pingout=$(ping -c 5 -W 2 "$target" 2>&1 || true)
  loss=$(printf '%s\n' "$pingout" | grep -o '[0-9]*% packet loss' | head -1 | awk '{print $1}')
  rtt=$(printf '%s\n' "$pingout" | awk -F'/' '/min\/avg\/max/{print $5" ms"}')
  [[ -z "$rtt" ]] && rtt="Unavailable"; [[ -z "$loss" ]] && loss="100%"
  if timeout 3 bash -c "</dev/tcp/$target/$tcp_port" 2>/dev/null; then tcp="Reachable"; else tcp="Blocked / Unreachable"; fi
  mtu="Unknown"
  for m in 1472 1464 1452 1440 1400 1360 1300; do if ping -c1 -W1 -M do -s "$m" "$target" >/dev/null 2>&1; then mtu=$((m+28)); break; fi; done
  printf "\n  %-16s : %s\n" "Average RTT" "$rtt"
  printf "  %-16s : %s\n" "Packet Loss" "$loss"
  printf "  %-16s : %s\n" "Path MTU" "$mtu"
  printf "  %-16s : %s (%s)\n" "TCP Reachability" "$tcp" "$tcp_port"
  local lossn=${loss%%%}; [[ "$lossn" =~ ^[0-9]+$ ]] || lossn=100
  if ((lossn==0)); then quality="Excellent"; elif ((lossn<=5)); then quality="Good"; elif ((lossn<=20)); then quality="Fair"; else quality="Poor"; fi
  printf "  %-16s : %s\n" "Estimated Quality" "$quality"
  echo; press_key
}

tunnel_notes() {
  local cfg="$1" notes="${cfg%.toml}.notes" choice
  clear; section_header "Tunnel Notes"
  echo "  Config : $(basename "$cfg")"; echo
  [[ -s "$notes" ]] && { echo "Current note:"; sed 's/^/  /' "$notes"; echo; } || echo "  No note saved."
  echo "  [1] Add / Edit note"; echo "  [2] Delete note"; echo "  [0] Back"; echo
  read -r -p "Select: " choice
  case "$choice" in
    1) echo "Enter note (finish with an empty line):"; : > "$notes"; while IFS= read -r line && [[ -n "$line" ]]; do printf '%s\n' "$line" >> "$notes"; done; colorize green "Note saved." ;;
    2) rm -f "$notes"; colorize green "Note deleted." ;;
  esac
  sleep 1
}

tunnel_health() {
  local cfg="$1" name service health_addr health_port peer active enabled restarts since timer
  name=$(basename "${cfg%.toml}"); service="tixotunnel-${name}.service"
  active=$(systemctl is-active "$service" 2>/dev/null || true); enabled=$(systemctl is-enabled "$service" 2>/dev/null || true)
  restarts=$(systemctl show "$service" -p NRestarts --value 2>/dev/null); since=$(systemctl show "$service" -p ActiveEnterTimestamp --value 2>/dev/null)
  health_addr=$(grep -E 'health.*=' "$cfg" | head -1 | cut -d'"' -f2); [[ -z "$health_addr" ]] && health_addr="10.10.10.1:101"
  health_port=${health_addr##*:}; health_host=${health_addr%:*}
  if timeout 2 bash -c "</dev/tcp/$health_host/$health_port" 2>/dev/null; then reach="Reachable"; else reach="Unavailable"; fi
  timer=$(systemctl show "${service%.service}-auto-restart.timer" -p NextElapseUSecRealtime --value 2>/dev/null)
  clear; section_header "Tunnel Health"
  printf "  %-17s : %s\n" "Tunnel" "$name"
  printf "  %-17s : %s\n" "Service" "$active"
  printf "  %-17s : %s\n" "Boot Enabled" "$enabled"
  printf "  %-17s : %s\n" "Health Endpoint" "$health_addr"
  printf "  %-17s : %s\n" "Health Port" "$reach"
  printf "  %-17s : %s\n" "Restart Count" "${restarts:-0}"
  printf "  %-17s : %s\n" "Running Since" "${since:-Unknown}"
  printf "  %-17s : %s\n" "Next Auto Restart" "${timer:-Disabled}"
  echo; journalctl -u "$service" -n 3 --no-pager -o cat 2>/dev/null | brand_engine_output | sed 's/^/  /'
  echo; press_key
}

view_service_logs() {
  local service="$1" mode choice
  clear; section_header "Live Log Filter"
  echo "  [1] All events"; echo "  [2] Errors only"; echo "  [3] Warnings + errors"; echo "  [4] Connection / health"; echo "  [0] Back"; echo
  read -r -p "Filter [1]: " choice; choice=${choice:-1}
  clear; show_live_header "$service"; colorize yellow "Press Ctrl+C to return"
  case "$choice" in
    2) journalctl -eu "$service" -f -o cat | brand_engine_output | grep --line-buffered -Ei 'error|failed|fatal|panic' ;;
    3) journalctl -eu "$service" -f -o cat | brand_engine_output | grep --line-buffered -Ei 'warn|error|failed|fatal|panic' ;;
    4) journalctl -eu "$service" -f -o cat | brand_engine_output | grep --line-buffered -Ei 'health|connect|listen|worker|packet|peer|bound|socket' ;;
    0) return ;;
    *) journalctl -eu "$service" -f -o cat | brand_engine_output ;;
  esac
}

spoof_result_summary() {
  local log="$1" passed="$2" failed="$3" csv="$4"
  awk '
  match($0,/([0-9]{1,3}\.){3}[0-9]{1,3}/){ip=substr($0,RSTART,RLENGTH)}
  match($0,/([0-9]+)\/([0-9]+)/,a){recv=a[1];total=a[2]; loss=(total?100-(recv*100/total):100); if(ip!="") print ip","recv","total","loss}
  ' "$log" | sort -u > "$csv" 2>/dev/null || true
  : > "$passed"; : > "$failed"
  while IFS=, read -r ip recv total loss; do [[ -z "$ip" ]] && continue; awk "BEGIN{exit !($loss <= 20)}" && echo "$ip" >> "$passed" || echo "$ip" >> "$failed"; done < "$csv"
}

spoof_tester_run() {
  install_spoof_tester || { press_key; return 1; }; mkdir -p "$SPOOF_TEST_DIR"
  clear; section_header "Advanced Spoof Tester"
  echo "  Start Receiver on Server 2, then Sender on Server 1."; echo "  Receiver stays live and grades each spoofed source address."; echo
  echo "  [1] Receiver  — wait, capture and grade"; echo "  [2] Sender    — transmit forged test packets"; echo "  [3] Browse previous results"; echo "  [0] Back"; echo
  read -r -p "Select role [0-3]: " role
  [[ "$role" == 0 ]] && return
  if [[ "$role" == 3 ]]; then clear; section_header "Spoof Test Archive"; ls -1t "$SPOOF_TEST_DIR" 2>/dev/null | head -30 | nl -w2 -s') '; echo; press_key; return; fi
  [[ "$role" == 1 || "$role" == 2 ]] || return
  mode=$([[ "$role" == 1 ]] && echo receiver || echo sender)
  clear; wizard_header "1/5" "PROTOCOL" "TCP uses a port; ICMP operates without a port"
  select_option "Protocol" "1" protocol "tcp" "icmp"
  dst_port=""
  if [[ "$protocol" == tcp && "$mode" == receiver ]]; then
    while true; do
      read -r -p "Filter Port [0 = all]: " dst_port
      dst_port=${dst_port:-0}
      if [[ "$dst_port" =~ ^[0-9]+$ ]] && (( dst_port >= 0 && dst_port <= 65535 )); then break; fi
      colorize red "Enter 0 for all ports or a valid port from 1 to 65535."
    done
  elif [[ "$protocol" == tcp && "$mode" == sender ]]; then
    while true; do
      read -r -p "Destination Port [1-65535]: " dst_port
      if [[ "$dst_port" =~ ^[0-9]+$ ]] && (( dst_port >= 1 && dst_port <= 65535 )); then break; fi
      colorize red "Destination Port is required and must be between 1 and 65535."
    done
  fi
  stamp=$(date +%Y%m%d-%H%M%S); src_file="$SPOOF_TEST_DIR/sources-$stamp.txt"; log="$SPOOF_TEST_DIR/${mode}-$stamp.log"
  clear; wizard_header "2/5" "SOURCE IP SET" "One IP, CIDR, range or an existing file"
  prepare_spoof_source_list "$src_file" || { rm -f "$src_file"; press_key; return; }
  clear; wizard_header "3/5" "TEST POLICY" "Tune packet count, loss threshold, timeout and concurrency"
  prompt_positive_int "Packets per IP" "10" packet_count
  read -r -p "Maximum packet loss % [20]: " max_loss; max_loss=${max_loss:-20}
  prompt_positive_int "Timeout seconds" "30" timeout
  concurrency=100; dst_ip=""
  if [[ "$mode" == sender ]]; then read -r -p "Receiver public IP: " dst_ip; prompt_positive_int "Concurrency" "100" concurrency; fi
  clear; wizard_header "4/5" "EXECUTION REVIEW" "Use identical protocol, port and packet policy on both servers"
  printf "  Role          : %s\n  Protocol      : %s\n" "${mode^^}" "$protocol"
  if [[ "$protocol" == tcp && "$mode" == receiver ]]; then
    [[ "$dst_port" == 0 ]] && printf "  Filter Port   : All ports\n" || printf "  Filter Port   : %s\n" "$dst_port"
  elif [[ "$protocol" == tcp && "$mode" == sender ]]; then
    printf "  Destination   : %s:%s\n" "$dst_ip" "$dst_port"
  fi
  printf "  Source entries: %s\n  Packets / IP  : %s\n  Max loss      : %s%%\n  Timeout       : %ss\n" "$(wc -l < "$src_file")" "$packet_count" "$max_loss" "$timeout"
  [[ "$mode" == sender ]] && printf "  Concurrency   : %s\n" "$concurrency"

  echo
  colorize cyan "Preflight checks" bold
  printf "  %-18s : %s\n" "Root privileges" "$([[ $EUID -eq 0 ]] && echo PASS || echo FAIL)"
  printf "  %-18s : %s\n" "Tester executable" "$([[ -x $SPOOF_TESTER_FILE ]] && echo PASS || echo FAIL)"
  printf "  %-18s : %s\n" "Source list" "$([[ -s $src_file ]] && echo PASS || echo FAIL)"
  if [[ "$mode" == sender ]]; then
    if valid_ipv4 "$dst_ip"; then
      printf "  %-18s : %s\n" "Receiver IPv4" "PASS ($dst_ip)"
    else
      printf "  %-18s : %s\n" "Receiver IPv4" "FAIL"
      colorize red "Receiver public IP must be a valid IPv4 address."
      rm -f "$src_file"; press_key; return 1
    fi
    local route_line route_iface route_src
    route_line=$(ip -4 route get "$dst_ip" 2>/dev/null | head -1)
    route_iface=$(awk '{for(i=1;i<=NF;i++) if($i=="dev") print $(i+1)}' <<< "$route_line")
    route_src=$(awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' <<< "$route_line")
    printf "  %-18s : %s\n" "Outbound route" "${route_iface:-Unavailable} / ${route_src:-Unknown}"
    [[ -n "$route_iface" ]] || { colorize red "No IPv4 route to receiver."; rm -f "$src_file"; press_key; return 1; }
  fi
  local source_total
  source_total=$(wc -l < "$src_file")
  if (( source_total > 4096 )); then
    colorize yellow "Large source set detected ($source_total entries). This can generate heavy traffic." bold
  fi
  echo
  colorize yellow "Use only on systems and networks you own or are explicitly authorized to test." bold
  read -r -p "Type START to begin: " confirm
  [[ "$confirm" == "START" ]] || { colorize yellow "Test cancelled."; rm -f "$src_file"; sleep 1; return; }

  local meta="$SPOOF_TEST_DIR/session-$stamp.txt"
  {
    echo "TixoTunnel Spoof Test Session"
    echo "Session: $stamp"
    echo "Role: $mode"
    echo "Protocol: $protocol"
    echo "Destination IP: ${dst_ip:-N/A}"
    echo "Destination Port: ${dst_port:-N/A}"
    echo "Source Entries: $source_total"
    echo "Packets per IP: $packet_count"
    echo "Maximum Loss: $max_loss%"
    echo "Timeout: ${timeout}s"
    echo "Concurrency: ${concurrency:-N/A}"
    echo "Started UTC: $(date -u +%FT%TZ)"
  } > "$meta"

  clear; wizard_header "5/5" "LIVE RESULTS" "Output is saved automatically; Ctrl+C stops the receiver"
  cmd=("$SPOOF_TESTER_FILE" tester --mode "$mode" --protocol "$protocol" --src-list "$src_file" --timeout "$timeout" --packet-count "$packet_count" --max-loss "$max_loss")
  [[ "$protocol" == tcp ]] && cmd+=(--dst-port "$dst_port")
  [[ "$mode" == sender ]] && cmd+=(--dst-ip "$dst_ip" --concurrency "$concurrency")
  "${cmd[@]}" 2>&1 | stdbuf -oL tee "$log" | while IFS= read -r line; do
    if [[ "$line" =~ PASS|passed|success ]]; then printf '\033[38;5;46m%s\033[0m\n' "$line"; elif [[ "$line" =~ FAIL|failed|loss|ERROR ]]; then printf '\033[38;5;196m%s\033[0m\n' "$line"; else echo "$line"; fi
  done
  rc=${PIPESTATUS[0]}; passed="$SPOOF_TEST_DIR/passed-$stamp.txt"; failed="$SPOOF_TEST_DIR/failed-$stamp.txt"; csv="$SPOOF_TEST_DIR/results-$stamp.csv"
  spoof_result_summary "$log" "$passed" "$failed" "$csv"
  echo; section_header "Result Files"
  printf "  Session  : %s\n  Full log : %s\n  Passed   : %s\n  Failed   : %s\n  CSV      : %s\n" "$meta" "$log" "$passed" "$failed" "$csv"
  printf "\n  Passed: %s   Failed: %s\n" "$(wc -l < "$passed")" "$(wc -l < "$failed")"
  ((rc==0)) && colorize green "Spoof test completed." bold || colorize yellow "Tester exited with status $rc; partial files were preserved."
  echo; press_key
}


ensure_iperf3() {
  command -v iperf3 >/dev/null 2>&1 && return 0
  clear; section_header "Link Benchmark Setup"
  colorize yellow "iperf3 is not installed. TixoTunnel will install it automatically." bold
  echo
  export DEBIAN_FRONTEND=noninteractive
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y && apt-get install -y iperf3
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y iperf3
  elif command -v yum >/dev/null 2>&1; then
    yum install -y iperf3
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm iperf3
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache iperf3
  else
    colorize red "No supported package manager was found. Install iperf3 manually." bold
    return 1
  fi
  command -v iperf3 >/dev/null 2>&1 || { colorize red "iperf3 installation failed." bold; return 1; }
  colorize green "iperf3 installed successfully." bold
  sleep 1
}

valid_port_prompt() {
  local label="$1" default="$2" __var="$3" value
  while true; do
    read -r -p "$label [$default]: " value; value=${value:-$default}
    if [[ "$value" =~ ^[0-9]+$ ]] && (( value >= 1 && value <= 65535 )); then
      printf -v "$__var" '%s' "$value"; return 0
    fi
    colorize red "Enter a valid port from 1 to 65535."
  done
}

link_benchmark_receiver() {
  ensure_iperf3 || { press_key; return; }
  mkdir -p "$LINK_TEST_DIR"
  clear; wizard_header "1/3" "RECEIVER" "Start a one-session iperf3 listener on the remote server"
  local port timeout_s bind_ip peer_ip stamp log pid
  valid_port_prompt "Listen port" "5201" port
  read -r -p "Bind address [0.0.0.0]: " bind_ip; bind_ip=${bind_ip:-0.0.0.0}
  prompt_positive_int "Receiver timeout seconds" "180" timeout_s
  read -r -p "Allowed peer IP (optional, informational): " peer_ip
  stamp=$(date +%Y%m%d-%H%M%S); log="$LINK_TEST_DIR/receiver-$stamp.log"
  clear; wizard_header "2/3" "RECEIVER REVIEW" "The listener closes after one completed test or when timeout expires"
  printf "  Listen address : %s\n  Port           : %s\n  Auto close     : One test\n  Timeout        : %ss\n  Allowed peer   : %s\n" "$bind_ip" "$port" "$timeout_s" "${peer_ip:-Any}"
  echo; read -r -p "Start receiver? [Y/n]: " ok; [[ "$ok" =~ ^[Nn] ]] && return
  clear; wizard_header "3/3" "RECEIVER LIVE" "Keep this terminal open while the other server starts the client"
  colorize green "● RECEIVER READY" bold
  printf "\n  Address : %s\n  Port    : %s\n  Timeout : %ss\n  Log     : %s\n\n" "$bind_ip" "$port" "$timeout_s" "$log"
  colorize yellow "Waiting for one benchmark session... Press Ctrl+C to cancel." bold
  timeout --signal=INT "$timeout_s" iperf3 -s -1 -B "$bind_ip" -p "$port" 2>&1 | tee "$log"
  local rc=${PIPESTATUS[0]}
  echo
  if (( rc == 0 )); then colorize green "Receiver session completed successfully." bold
  elif (( rc == 124 )); then colorize yellow "Receiver timed out without a completed test." bold
  else colorize yellow "Receiver stopped with status $rc. The log was preserved." bold; fi
  echo; press_key
}

iperf_live_summary() {
  local raw="$1" direction="$2" protocol="$3" duration="$4" streams="$5" txt="$6" json="$7"
  python3 - "$raw" "$direction" "$protocol" "$duration" "$streams" "$txt" "$json" <<'PYI'
import re,sys,json,datetime,os
raw,direction,protocol,duration,streams,txt,json_out=sys.argv[1:]
text=open(raw,errors='replace').read()
lines=text.splitlines()

def to_bps(value, unit):
    v=float(value)
    m={'bits/sec':1,'Kbits/sec':1e3,'Mbits/sec':1e6,'Gbits/sec':1e9,'Tbits/sec':1e12}
    return v*m.get(unit,1)

def to_bytes(value, unit):
    v=float(value)
    m={'Bytes':1,'KBytes':1e3,'MBytes':1e6,'GBytes':1e9,'TBytes':1e12}
    return v*m.get(unit,1)

def human_rate(v):
    units=['bit/s','Kbit/s','Mbit/s','Gbit/s','Tbit/s']; i=0
    while v>=1000 and i<len(units)-1: v/=1000; i+=1
    return f'{v:.2f} {units[i]}'

def human_size(v):
    units=['B','KB','MB','GB','TB']; i=0
    while v>=1000 and i<len(units)-1: v/=1000; i+=1
    return f'{v:.2f} {units[i]}'

# iperf final lines contain transfer and bitrate; prefer receiver for TCP.
pat=re.compile(r'(?P<transfer>[0-9.]+)\s+(?P<tunit>[KMGTP]?Bytes)\s+(?P<rate>[0-9.]+)\s+(?P<runit>[KMGTP]?bits/sec)(?:\s+(?P<retr>[0-9]+))?(?:\s+(?P<jitter>[0-9.]+)\s+ms\s+(?P<lost>[0-9]+)/(?P<total>[0-9]+)\s+\((?P<loss>[0-9.]+)%\))?')
records=[]
for line in lines:
    m=pat.search(line)
    if not m: continue
    d=m.groupdict(); d['line']=line
    d['bps']=to_bps(d['rate'],d['runit']); d['bytes']=to_bytes(d['transfer'],d['tunit'])
    records.append(d)

finals=[r for r in records if '0.00-' in r['line'] and ('receiver' in r['line'] or 'sender' in r['line'])]
if protocol.upper()=='UDP':
    candidates=[r for r in records if r.get('loss') is not None]
    main=candidates[-1] if candidates else (records[-1] if records else None)
else:
    receivers=[r for r in finals if 'receiver' in r['line']]
    main=receivers[-1] if receivers else (finals[-1] if finals else (records[-1] if records else None))

if not main:
    open(txt,'w').write('Unable to parse iperf3 live output. See raw log:\n'+raw+'\n')
    json.dump({'error':'parse_failed','raw_log':raw},open(json_out,'w'),indent=2)
    print(open(txt).read())
    sys.exit(1)

bps=main['bps']; bytes_n=main['bytes']
retrans=main.get('retr') or 'N/A'
jitter=main.get('jitter') or 'N/A'
loss=main.get('loss') or 'N/A'
mbps=bps/1e6
if protocol.upper()=='UDP':
    lp=float(loss) if loss!='N/A' else 100.0
    quality='Excellent' if lp<0.5 else 'Good' if lp<2 else 'Fair' if lp<5 else 'Poor'
else:
    try: r=int(retrans)
    except: r=0
    quality='Excellent' if mbps>=500 and r<100 else 'Good' if mbps>=100 else 'Fair' if mbps>=20 else 'Poor'

result={
  'direction':direction,'protocol':protocol.upper(),'duration_seconds':int(duration),
  'parallel_streams':int(streams),'average_bits_per_second':bps,
  'average_speed':human_rate(bps),'transferred_bytes':bytes_n,
  'transferred':human_size(bytes_n),'retransmits':retrans,
  'jitter_ms':jitter,'packet_loss_percent':loss,'quality':quality,
  'generated_at':datetime.datetime.now().isoformat(timespec='seconds'),'raw_log':raw
}
json.dump(result,open(json_out,'w'),indent=2)
out=[
'────────────────────────────────────────────────────────────────────',
' LINK BENCHMARK COMPLETE',
'────────────────────────────────────────────────────────────────────','',
f'  Direction      : {direction}',f'  Protocol       : {protocol.upper()}',
f'  Duration       : {duration} seconds',f'  Parallel Flows : {streams}','',
f'  Average Speed  : {result["average_speed"]}',f'  Transferred    : {result["transferred"]}',
]
if protocol.upper()=='UDP': out += [f'  Jitter         : {jitter} ms',f'  Packet Loss    : {loss}%']
else: out += [f'  Retransmits    : {retrans}']
out += [f'  Link Quality   : {quality}','',f'  Generated      : {result["generated_at"]}']
summary='\n'.join(out)+'\n'
open(txt,'w').write(summary)
print(summary)
PYI
}

link_benchmark_client() {
  ensure_iperf3 || { press_key; return; }
  mkdir -p "$LINK_TEST_DIR"
  clear; wizard_header "1/4" "TEST MODE" "Measure real bandwidth between this server and the receiver"
  local mode server_ip port duration streams udp_rate direction protocol extra=() stamp json txt raw
  select_option "Benchmark mode" "1" mode "TCP Upload (this server → receiver)" "TCP Download (receiver → this server)" "TCP Bidirectional" "UDP Quality"
  read -r -p "Receiver IP or hostname: " server_ip; [[ -n "$server_ip" ]] || return
  valid_port_prompt "Receiver port" "5201" port
  clear; wizard_header "2/4" "TEST POLICY" "Longer tests and parallel flows can reveal the real link ceiling"
  prompt_positive_int "Duration seconds" "10" duration
  prompt_positive_int "Parallel streams" "4" streams
  case "$mode" in
    "TCP Upload (this server → receiver)") protocol=tcp; direction="LOCAL → REMOTE" ;;
    "TCP Download (receiver → this server)") protocol=tcp; direction="REMOTE → LOCAL"; extra+=(--reverse) ;;
    "TCP Bidirectional") protocol=tcp; direction="LOCAL ↔ REMOTE"; extra+=(--bidir) ;;
    "UDP Quality")
      protocol=udp; direction="LOCAL → REMOTE"
      read -r -p "Target UDP bandwidth [100M]: " udp_rate; udp_rate=${udp_rate:-100M}
      extra+=(-u -b "$udp_rate") ;;
  esac
  clear; wizard_header "3/4" "EXECUTION REVIEW" "The remote receiver must already be waiting on the same port"
  printf "  Receiver       : %s:%s\n  Direction      : %s\n  Protocol       : %s\n  Duration       : %ss\n  Parallel flows : %s\n" "$server_ip" "$port" "$direction" "${protocol^^}" "$duration" "$streams"
  [[ "$protocol" == udp ]] && printf "  UDP target     : %s\n" "$udp_rate"
  echo; read -r -p "Start benchmark? [Y/n]: " ok; [[ "$ok" =~ ^[Nn] ]] && return
  stamp=$(date +%Y%m%d-%H%M%S); json="$LINK_TEST_DIR/result-$stamp.json"; txt="$LINK_TEST_DIR/result-$stamp.txt"; raw="$LINK_TEST_DIR/raw-$stamp.log"
  clear; wizard_header "4/4" "LIVE LINK BENCHMARK" "One-second intervals are displayed live and saved at the same time"
  printf "  Target         : %s:%s\n  Direction      : %s\n  Protocol       : %s\n  Duration       : %ss\n  Parallel flows : %s\n  Raw log        : %s\n\n" "$server_ip" "$port" "$direction" "${protocol^^}" "$duration" "$streams" "$raw"
  colorize green "● TEST STARTED — live intervals follow" bold
  echo
  local cmd=(iperf3 -c "$server_ip" -p "$port" -t "$duration" -P "$streams" -i 1 --forceflush)
  cmd+=("${extra[@]}")
  "${cmd[@]}" 2>&1 | tee "$raw"
  local rc=${PIPESTATUS[0]}
  echo
  if (( rc != 0 )); then
    colorize red "Benchmark failed (status $rc)." bold
    printf "\nRaw log: %s\n" "$raw"; echo; press_key; return
  fi
  iperf_live_summary "$raw" "$direction" "$protocol" "$duration" "$streams" "$txt" "$json" || true
  printf "\n  Text summary : %s\n  JSON result  : %s\n  Raw output   : %s\n" "$txt" "$json" "$raw"
  echo; press_key
}

link_benchmark_archive() {
  mkdir -p "$LINK_TEST_DIR"
  clear; section_header "Link Benchmark Archive"
  local files=("$LINK_TEST_DIR"/result-*.txt)
  if [[ ! -e "${files[0]}" ]]; then echo "  No saved benchmark results."; echo; press_key; return; fi
  local i=1 f choice
  for f in $(ls -1t "$LINK_TEST_DIR"/result-*.txt 2>/dev/null); do printf "  [%d] %s\n" "$i" "$(basename "$f")"; ((i++)); done
  echo; read -r -p "Open result [0 = back]: " choice; [[ "$choice" == 0 ]] && return
  [[ "$choice" =~ ^[0-9]+$ ]] || return
  f=$(ls -1t "$LINK_TEST_DIR"/result-*.txt 2>/dev/null | sed -n "${choice}p")
  [[ -f "$f" ]] || return
  clear; cat "$f"; echo; press_key
}

link_benchmark_menu() {
  clear; section_header "Link Benchmark"
  echo "  Measure actual TCP throughput and UDP quality between two servers."
  echo
  echo "  [1] Start Receiver (remote server)"
  echo "  [2] Run Client Test (local server)"
  echo "  [3] View Saved Results"
  echo "  [0] Back"
  echo
  read -r -p "Select: " b
  case "$b" in 1) link_benchmark_receiver;; 2) link_benchmark_client;; 3) link_benchmark_archive;; esac
}


ensure_server_speedtest() {
  if command -v speedtest >/dev/null 2>&1 || command -v speedtest-cli >/dev/null 2>&1; then return 0; fi
  clear; section_header "Server Speed Test Setup"
  colorize yellow "No speed-test client was found. TixoTunnel will install one automatically." bold
  export DEBIAN_FRONTEND=noninteractive
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y && apt-get install -y speedtest-cli
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y speedtest-cli || dnf install -y python3-pip && pip3 install speedtest-cli
  elif command -v yum >/dev/null 2>&1; then
    yum install -y speedtest-cli || yum install -y python3-pip && pip3 install speedtest-cli
  elif command -v pacman >/dev/null 2>&1; then
    pacman -Sy --noconfirm speedtest-cli
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache py3-speedtest-cli || apk add --no-cache py3-pip && pip3 install speedtest-cli
  else
    colorize red "No supported package manager was found."; return 1
  fi
  command -v speedtest >/dev/null 2>&1 || command -v speedtest-cli >/dev/null 2>&1
}

server_speedtest_run() {
  ensure_server_speedtest || { colorize red "Speed-test installation failed."; press_key; return; }
  mkdir -p "$SPEEDTEST_DIR"
  local stamp raw json txt cmd rc
  stamp=$(date +%Y%m%d-%H%M%S)
  raw="$SPEEDTEST_DIR/raw-$stamp.log"; json="$SPEEDTEST_DIR/result-$stamp.json"; txt="$SPEEDTEST_DIR/result-$stamp.txt"
  clear; section_header "Server Speed Test"
  printf "  Public IP : %s\n  Started   : %s\n\n" "$(get_public_ip)" "$(date -Is)"
  colorize cyan "Selecting the best public test server..." bold
  echo "Testing latency, download and upload. This may take a minute."
  echo
  if command -v speedtest >/dev/null 2>&1 && speedtest --help 2>&1 | grep -q -- '--format'; then
    cmd=(speedtest --accept-license --accept-gdpr --progress=yes)
    "${cmd[@]}" 2>&1 | tee "$raw"
    rc=${PIPESTATUS[0]}
    if ((rc==0)); then speedtest --accept-license --accept-gdpr --format=json >"$json" 2>/dev/null || true; fi
  else
    cmd=(speedtest-cli --secure --simple)
    "${cmd[@]}" 2>&1 | tee "$raw"
    rc=${PIPESTATUS[0]}
    if ((rc==0)); then speedtest-cli --secure --json >"$json" 2>/dev/null || true; fi
  fi
  ((rc==0)) || { colorize red "Server speed test failed. Raw log: $raw"; press_key; return; }
  {
    echo "TixoTunnel Server Speed Test"
    echo "Generated: $(date -Is)"
    echo "Public IP: $(get_public_ip)"
    echo
    cat "$raw"
  } > "$txt"
  echo; colorize green "Server speed test completed." bold
  printf "\n  Text result : %s\n  JSON result : %s\n  Raw output  : %s\n" "$txt" "$json" "$raw"
  echo; press_key
}

server_speedtest_archive() {
  mkdir -p "$SPEEDTEST_DIR"; clear; section_header "Server Speed Test Archive"
  local files=("$SPEEDTEST_DIR"/result-*.txt) i=1 f choice
  if [[ ! -e "${files[0]}" ]]; then echo "  No saved speed-test results."; echo; press_key; return; fi
  while IFS= read -r f; do printf "  [%d] %s\n" "$i" "$(basename "$f")"; ((i++)); done < <(ls -1t "$SPEEDTEST_DIR"/result-*.txt 2>/dev/null)
  echo; read -r -p "Open result [0 = back]: " choice; [[ "$choice" == 0 ]] && return
  [[ "$choice" =~ ^[0-9]+$ ]] || return
  f=$(ls -1t "$SPEEDTEST_DIR"/result-*.txt 2>/dev/null | sed -n "${choice}p")
  [[ -f "$f" ]] || return; clear; cat "$f"; echo; press_key
}

server_speedtest_menu() {
  clear; section_header "Server Speed Test"
  echo "  Test the public Internet connection of this server."
  echo "  This is separate from Link Benchmark, which tests server-to-server tunnel capacity."
  echo
  echo "  [1] Run Full Speed Test"
  echo "  [2] View Saved Results"
  echo "  [0] Back"
  echo
  read -r -p "Select: " s
  case "$s" in 1) server_speedtest_run;; 2) server_speedtest_archive;; esac
}

ensure_diagnostic_tools() {
  local missing=() p
  for p in ip ping ss awk sed grep tar python3; do command -v "$p" >/dev/null 2>&1 || missing+=("$p"); done
  if ((${#missing[@]})); then
    colorize yellow "Installing diagnostic dependencies..." bold
    if command -v apt-get >/dev/null; then apt-get update -y && DEBIAN_FRONTEND=noninteractive apt-get install -y iproute2 iputils-ping procps python3 tar curl
    elif command -v dnf >/dev/null; then dnf install -y iproute iputils procps-ng python3 tar curl
    elif command -v yum >/dev/null; then yum install -y iproute iputils procps-ng python3 tar curl
    elif command -v apk >/dev/null; then apk add --no-cache iproute2 iputils procps python3 tar curl
    else colorize red "Unsupported package manager."; return 1; fi
  fi
}
first_tunnel_service() { systemctl list-unit-files 'tixotunnel-*.service' --no-legend 2>/dev/null | awk 'NR==1{print $1}'; }
first_peer_from_configs() { grep -hE '^[[:space:]]*(remote_addr|remote_ip|address)[[:space:]]*=' "$config_dir"/*.toml 2>/dev/null | head -1 | sed -E 's/.*=[[:space:]]*"?([^": ]+).*/\1/'; }
network_snapshot_create_silent() { local out="$1"; { echo "Generated: $(date -Is)"; uname -a; uptime; free -h; df -h /; ip -brief address; ip route; cat /etc/resolv.conf 2>/dev/null; ss -lntup 2>/dev/null; } >"$out"; }
network_snapshot_create() {
  ensure_diagnostic_tools || { press_key; return; }; mkdir -p "$SNAPSHOT_DIR"
  local out="$SNAPSHOT_DIR/network-$(date +%Y%m%d-%H%M%S).txt"; network_snapshot_create_silent "$out"
  clear; section_header "Network Snapshot"; colorize green "Snapshot created." bold; printf "\n  File: %s\n\n" "$out"; press_key
}
log_analyzer() {
  clear; section_header "Log Analyzer"; local service data
  service=$(first_tunnel_service); [[ -n "$service" ]] || { colorize yellow "No tunnel service found."; press_key; return; }
  read -r -p "Service [$service]: " input; service=${input:-$service}; data=$(journalctl -u "$service" --since '24 hours ago' --no-pager -o cat 2>/dev/null || true)
  printf "\n  Errors/Fatal         : %s\n" "$(grep -Eic 'error|fatal|panic|failed' <<<"$data")"
  printf "  Warnings             : %s\n" "$(grep -Eic 'warn|warning' <<<"$data")"
  printf "  Timeouts             : %s\n" "$(grep -Eic 'timeout|timed out' <<<"$data")"
  printf "  Refused/Unreachable  : %s\n" "$(grep -Eic 'refused|unreachable' <<<"$data")"
  printf "  Resets/Broken Pipe   : %s\n" "$(grep -Eic 'reset by peer|broken pipe' <<<"$data")"
  printf "  TLS/Auth Issues      : %s\n\n" "$(grep -Eic 'auth|certificate|tls|handshake' <<<"$data")"
  section_header "Recent Important Events"; grep -Ei 'error|fatal|panic|failed|warn|timeout|refused|unreachable|reset|broken pipe|certificate|handshake' <<<"$data" | tail -20 | brand_engine_output
  echo; press_key
}
diagnostic_collect() {
  local service="$1" peer="$2" port="$3" report="$4" score=100 active enabled peer_ok=0 port_ok=0 route_ok=0 dns_ok=0 loss="N/A" rtt="N/A" mtu="N/A" errors=0 firewall
  systemctl is-active --quiet "$service" && active="Running" || { active="Stopped"; score=$((score-35)); }
  systemctl is-enabled --quiet "$service" 2>/dev/null && enabled="Enabled" || { enabled="Disabled"; score=$((score-5)); }
  if [[ -n "$peer" ]]; then
    local po; po=$(ping -c 4 -W 2 "$peer" 2>&1 || true); loss=$(grep -o '[0-9]*% packet loss' <<<"$po" | head -1 | awk '{print $1}'); rtt=$(awk -F'/' '/min\/avg\/max/{print $5" ms"}' <<<"$po")
    [[ -n "$loss" && "$loss" != "100%" ]] && peer_ok=1 || score=$((score-20))
    for m in 1472 1464 1452 1440 1400 1360 1300; do ping -c1 -W1 -M do -s "$m" "$peer" >/dev/null 2>&1 && { mtu=$((m+28)); break; }; done
    if [[ "$port" =~ ^[0-9]+$ ]] && timeout 3 bash -c "</dev/tcp/$peer/$port" 2>/dev/null; then port_ok=1; fi
  fi
  ip route get "${peer:-1.1.1.1}" >/dev/null 2>&1 && route_ok=1 || { score=$((score-10)); }
  getent hosts github.com >/dev/null 2>&1 && dns_ok=1 || { score=$((score-5)); }
  command -v nft >/dev/null && firewall="nftables" || { command -v iptables >/dev/null && firewall="iptables" || firewall="Not detected"; }
  errors=$(journalctl -u "$service" --since '1 hour ago' --no-pager -o cat 2>/dev/null | grep -Eic 'error|fatal|panic|failed'); ((errors>0)) && score=$((score-10)); ((score<0)) && score=0
  {
    echo "TixoTunnel Diagnostic Report"; echo "Generated: $(date -Is)"; echo "Version: $SCRIPT_VERSION"; echo
    printf "Service: %s\nState: %s\nBoot: %s\nPeer: %s\nPeer Reachable: %s\nPort: %s\nTCP Reachable: %s\nPacket Loss: %s\nAverage RTT: %s\nPath MTU: %s\nRoute: %s\nDNS: %s\nFirewall: %s\nRecent Errors: %s\nHealth Score: %s/100\n" "$service" "$active" "$enabled" "${peer:-Not configured}" "$peer_ok" "${port:-Skipped}" "$port_ok" "${loss:-N/A}" "${rtt:-N/A}" "$mtu" "$route_ok" "$dns_ok" "$firewall" "$errors" "$score"
    echo; systemctl status "$service" --no-pager 2>&1 || true; echo; journalctl -u "$service" --since '1 hour ago' --no-pager -o short-iso 2>/dev/null | tail -100
  } >"$report"
  DIAG_SCORE=$score; DIAG_ACTIVE=$active; DIAG_ENABLED=$enabled; DIAG_PEER_OK=$peer_ok; DIAG_PORT_OK=$port_ok; DIAG_ROUTE_OK=$route_ok; DIAG_DNS_OK=$dns_ok; DIAG_LOSS=${loss:-N/A}; DIAG_RTT=${rtt:-N/A}; DIAG_MTU=$mtu; DIAG_ERRORS=$errors
}
diagnostic_auto_fix() {
  local service="$1"; clear; section_header "Recommended Safe Fixes"
  echo "  [1] Restart service"; echo "  [2] Enable at boot"; echo "  [3] Reload systemd + restart"; echo "  [4] Flush DNS cache"; echo "  [5] Apply all safe fixes"; echo "  [0] Back"; echo
  read -r -p "Select: " f
  case "$f" in 1) systemctl restart "$service";; 2) systemctl enable "$service";; 3) systemctl daemon-reload; systemctl restart "$service";; 4) command -v resolvectl >/dev/null && resolvectl flush-caches || true;; 5) systemctl daemon-reload; systemctl enable "$service"; systemctl restart "$service"; command -v resolvectl >/dev/null && resolvectl flush-caches || true;; *) return;; esac
  colorize green "Safe fix completed." bold; sleep 1
}
diagnostic_center() {
  ensure_diagnostic_tools || { press_key; return; }; mkdir -p "$DIAG_DIR"; clear; section_header "Tunnel Diagnostic Center"
  local service peer port report input; service=$(first_tunnel_service); [[ -n "$service" ]] || { colorize yellow "No tunnel service found."; press_key; return; }
  read -r -p "Service [$service]: " input; service=${input:-$service}; peer=$(first_peer_from_configs); read -r -p "Peer IP [${peer:-skip}]: " input; peer=${input:-$peer}; read -r -p "Health/TCP port [skip]: " port
  report="$DIAG_DIR/diagnostic-$(date +%Y%m%d-%H%M%S).txt"; clear; section_header "Running Diagnostics"; diagnostic_collect "$service" "$peer" "$port" "$report"
  clear; section_header "Diagnostic Result"
  printf "  Service        : %s\n  Boot           : %s\n  Peer           : %s\n  TCP Port       : %s\n  Route          : %s\n  DNS            : %s\n  Packet Loss    : %s\n  Average RTT    : %s\n  Path MTU       : %s\n  Recent Errors  : %s\n\n" "$DIAG_ACTIVE" "$DIAG_ENABLED" "$([[ $DIAG_PEER_OK == 1 ]] && echo Reachable || echo Unreachable)" "$([[ $DIAG_PORT_OK == 1 ]] && echo Reachable || echo 'Skipped/Blocked')" "$([[ $DIAG_ROUTE_OK == 1 ]] && echo OK || echo Failed)" "$([[ $DIAG_DNS_OK == 1 ]] && echo OK || echo Failed)" "$DIAG_LOSS" "$DIAG_RTT" "$DIAG_MTU" "$DIAG_ERRORS"
  if ((DIAG_SCORE>=90)); then colorize green "  Overall Health : $DIAG_SCORE/100 — HEALTHY" bold; elif ((DIAG_SCORE>=65)); then colorize yellow "  Overall Health : $DIAG_SCORE/100 — ATTENTION" bold; else colorize red "  Overall Health : $DIAG_SCORE/100 — CRITICAL" bold; fi
  printf "\n  Report: %s\n\n  [1] Apply Safe Fixes\n  [2] Open Full Report\n  [0] Back\n\n" "$report"; read -r -p "Select: " a
  case "$a" in 1) diagnostic_auto_fix "$service";; 2) clear; less "$report" 2>/dev/null || cat "$report"; press_key;; esac
}
format_bytes() {
  local bytes="${1:-0}"
  awk -v b="$bytes" 'BEGIN {
    split("B KiB MiB GiB TiB PiB",u," "); i=1;
    while (b>=1024 && i<6) { b/=1024; i++ }
    if (i==1) printf "%.0f %s",b,u[i]; else printf "%.2f %s",b,u[i]
  }'
}
format_rate() {
  local bytes="${1:-0}"
  awk -v b="$bytes" 'BEGIN {
    bits=b*8;
    if (bits>=1000000000) printf "%.2f Gbit/s",bits/1000000000;
    else if (bits>=1000000) printf "%.2f Mbit/s",bits/1000000;
    else if (bits>=1000) printf "%.2f Kbit/s",bits/1000;
    else printf "%.0f bit/s",bits
  }'
}
percent_bar() {
  local value="${1:-0}" width="${2:-20}" filled empty
  value=${value%.*}; ((value<0)) && value=0; ((value>100)) && value=100
  filled=$((value*width/100)); empty=$((width-filled))
  printf '%*s' "$filled" '' | tr ' ' '■'
  printf '%*s' "$empty" '' | tr ' ' '□'
}
read_cpu_sample() {
  awk '/^cpu /{idle=$5+$6; total=0; for(i=2;i<=NF;i++) total+=$i; print total,idle; exit}' /proc/stat
}
get_iface_counters() {
  local iface="$1"
  awk -v i="$iface" '$1==i":" {gsub(":","",$1); print $2,$10,$3,$4,$11,$12; exit}' /proc/net/dev
}
health_label() {
  local score="$1"
  if ((score>=90)); then printf 'EXCELLENT'; elif ((score>=75)); then printf 'GOOD'; elif ((score>=55)); then printf 'ATTENTION'; else printf 'CRITICAL'; fi
}
live_dashboard() {
  ensure_diagnostic_tools || { press_key; return; }
  local service peer iface key status service_uptime engine_pid_count engine_cpu engine_rss
  local ctot1 cidle1 ctot2 cidle2 cpu_pct cpu_model cpu_cores cpu_threads cpu_mhz
  local rx1 tx1 rx2 tx2 rx_rate tx_rate total_rate rx_total tx_total traffic_total
  local rx_packets tx_packets rx_err rx_drop tx_err tx_drop
  local mem_total mem_avail mem_used mem_pct cached swap_total swap_free swap_used swap_pct
  local disk_total disk_used disk_free disk_pct load1 load5 load15 uptime_text
  local rtt loss jitter pingout connections established listening health score_label

  service=$(first_tunnel_service)
  peer=$(first_peer_from_configs)
  iface=$(ip route show default 2>/dev/null | awk 'NR==1{print $5}')
  [[ -n "$iface" ]] || iface=$(ls /sys/class/net 2>/dev/null | grep -v '^lo$' | head -1)
  [[ -n "$service" ]] || { colorize yellow "No tunnel service found."; press_key; return; }
  [[ -n "$iface" ]] || { colorize red "No network interface was detected."; press_key; return; }

  cpu_model=$(awk -F: '/model name|Hardware|Processor/{gsub(/^[ \t]+/,"",$2); print $2; exit}' /proc/cpuinfo)
  cpu_cores=$(awk -F: '/cpu cores/{gsub(/ /,"",$2); print $2; exit}' /proc/cpuinfo)
  cpu_threads=$(getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 1)
  [[ -n "$cpu_cores" ]] || cpu_cores="$cpu_threads"

  while true; do
    read -r ctot1 cidle1 < <(read_cpu_sample)
    read -r rx1 tx1 _ _ _ _ < <(get_iface_counters "$iface")
    sleep 1
    read -r ctot2 cidle2 < <(read_cpu_sample)
    read -r rx2 tx2 rx_packets rx_err tx_packets tx_err < <(get_iface_counters "$iface")

    cpu_pct=$(awk -v t1="$ctot1" -v i1="$cidle1" -v t2="$ctot2" -v i2="$cidle2" 'BEGIN{dt=t2-t1; di=i2-i1; if(dt>0) printf "%.1f",100*(dt-di)/dt; else print "0.0"}')
    rx_rate=$((rx2-rx1)); tx_rate=$((tx2-tx1)); ((rx_rate<0)) && rx_rate=0; ((tx_rate<0)) && tx_rate=0
    total_rate=$((rx_rate+tx_rate)); rx_total=$rx2; tx_total=$tx2; traffic_total=$((rx_total+tx_total))

    read -r _ _ rx_drop _ _ _ _ _ _ tx_drop _ _ _ _ _ _ < <(awk -v i="$iface" '$1==i":" {gsub(":","",$1); for(n=2;n<=NF;n++) printf "%s%s",$n,(n==NF?ORS:OFS)}' /proc/net/dev)
    rx_drop=${rx_drop:-0}; tx_drop=${tx_drop:-0}; rx_err=${rx_err:-0}; tx_err=${tx_err:-0}

    mem_total=$(awk '/MemTotal/{print $2*1024}' /proc/meminfo)
    mem_avail=$(awk '/MemAvailable/{print $2*1024}' /proc/meminfo)
    cached=$(awk '/^Cached:/{print $2*1024}' /proc/meminfo)
    mem_used=$((mem_total-mem_avail)); mem_pct=$(awk -v u="$mem_used" -v t="$mem_total" 'BEGIN{printf "%.1f",t?u*100/t:0}')
    swap_total=$(awk '/SwapTotal/{print $2*1024}' /proc/meminfo); swap_free=$(awk '/SwapFree/{print $2*1024}' /proc/meminfo)
    swap_used=$((swap_total-swap_free)); swap_pct=$(awk -v u="$swap_used" -v t="$swap_total" 'BEGIN{printf "%.1f",t?u*100/t:0}')

    read -r disk_total disk_used disk_free disk_pct < <(df -B1 --output=size,used,avail,pcent / 2>/dev/null | awk 'NR==2{gsub(/%/,"",$4); print $1,$2,$3,$4}')
    read -r load1 load5 load15 _ < /proc/loadavg
    uptime_text=$(uptime -p 2>/dev/null | sed 's/^up //')
    cpu_mhz=$(awk -F: '/cpu MHz/{gsub(/^[ \t]+/,"",$2); printf "%.0f MHz",$2; exit}' /proc/cpuinfo)

    systemctl is-active --quiet "$service" && status="RUNNING" || status="STOPPED"
    service_uptime=$(systemctl show "$service" -p ActiveEnterTimestamp --value 2>/dev/null)
    engine_pid_count=$(pgrep -fc 'tixotunnel-core' 2>/dev/null || echo 0)
    engine_cpu=$(ps -C tixotunnel-core -o %cpu= 2>/dev/null | awk '{s+=$1}END{printf "%.1f",s+0}')
    engine_rss=$(ps -C tixotunnel-core -o rss= 2>/dev/null | awk '{s+=$1}END{printf "%.0f",s*1024}')
    connections=$(ss -Htun 2>/dev/null | wc -l); established=$(ss -Htn state established 2>/dev/null | wc -l); listening=$(ss -Hlnt 2>/dev/null | wc -l)

    if [[ -n "$peer" ]]; then
      pingout=$(ping -c3 -W1 "$peer" 2>/dev/null || true)
      loss=$(sed -n 's/.*\([0-9][0-9]*\)% packet loss.*/\1/p' <<<"$pingout" | tail -1); loss=${loss:-100}
      read -r rtt jitter < <(awk -F'=' '/rtt|round-trip/{gsub(/ ms/,"",$2); split($2,a,"/"); printf "%.2f %.2f",a[2],a[4]}' <<<"$pingout")
      rtt=${rtt:-N/A}; jitter=${jitter:-N/A}
    else rtt="N/A"; jitter="N/A"; loss="N/A"; fi

    health=100
    [[ "$status" == RUNNING ]] || health=$((health-35))
    awk -v v="$cpu_pct" 'BEGIN{exit !(v>=90)}' && health=$((health-15))
    awk -v v="$mem_pct" 'BEGIN{exit !(v>=90)}' && health=$((health-15))
    ((disk_pct>=90)) && health=$((health-15))
    [[ "$loss" =~ ^[0-9]+$ ]] && ((loss>=10)) && health=$((health-15))
    ((rx_err+tx_err+rx_drop+tx_drop>0)) && health=$((health-5))
    ((health<0)) && health=0; score_label=$(health_label "$health")

    clear
    printf '\033[38;5;51m══════════════════════════════════════════════════════════════════════════\033[0m\n'
    printf '\033[97m TixoTunnel Resource & Traffic Monitor\033[0m  \033[38;5;245m%s | %s\033[0m\n' "$SCRIPT_VERSION" "$(date '+%Y-%m-%d %H:%M:%S')"
    printf '\033[38;5;51m══════════════════════════════════════════════════════════════════════════\033[0m\n'
    printf '  Service          : %-29s Health      : %s/100 %s\n' "$service" "$health" "$score_label"
    printf '  Tunnel           : %-29s Interface   : %s\n' "$status" "$iface"
    printf '  Peer             : %-29s Uptime      : %s\n' "${peer:-Not configured}" "${uptime_text:-N/A}"
    printf '  Active Since     : %s\n' "${service_uptime:-N/A}"

    section_header "SYSTEM RESOURCES"
    printf '  CPU              : %5s%%  [%s]\n' "$cpu_pct" "$(percent_bar "$cpu_pct" 20)"
    printf '  CPU Model        : %s\n' "${cpu_model:-Unknown}"
    printf '  CPU Layout       : %s cores / %s threads @ %s\n' "$cpu_cores" "$cpu_threads" "${cpu_mhz:-N/A}"
    printf '  Load Average     : %s / %s / %s\n' "$load1" "$load5" "$load15"
    printf '  RAM              : %s / %s (%s%%)  [%s]\n' "$(format_bytes "$mem_used")" "$(format_bytes "$mem_total")" "$mem_pct" "$(percent_bar "$mem_pct" 20)"
    printf '  RAM Available    : %-18s Cache       : %s\n' "$(format_bytes "$mem_avail")" "$(format_bytes "$cached")"
    printf '  Swap             : %s / %s (%s%%)\n' "$(format_bytes "$swap_used")" "$(format_bytes "$swap_total")" "$swap_pct"
    printf '  Disk             : %s / %s (%s%%)  [%s]\n' "$(format_bytes "$disk_used")" "$(format_bytes "$disk_total")" "$disk_pct" "$(percent_bar "$disk_pct" 20)"
    printf '  Disk Free        : %s\n' "$(format_bytes "$disk_free")"

    section_header "LIVE BANDWIDTH"
    printf '  Download (RX)    : %-18s Upload (TX) : %s\n' "$(format_rate "$rx_rate")" "$(format_rate "$tx_rate")"
    printf '  Combined         : %s\n' "$(format_rate "$total_rate")"
    printf '  RX Total         : %-18s TX Total    : %s\n' "$(format_bytes "$rx_total")" "$(format_bytes "$tx_total")"
    printf '  Total Transfer   : %s\n' "$(format_bytes "$traffic_total")"
    printf '  Packets          : RX %-14s TX %s\n' "$rx_packets" "$tx_packets"
    printf '  Errors           : RX %-14s TX %s\n' "$rx_err" "$tx_err"
    printf '  Dropped          : RX %-14s TX %s\n' "$rx_drop" "$tx_drop"

    section_header "TUNNEL PROCESS & CONNECTIVITY"
    printf '  Engine Processes : %-18s Engine CPU  : %s%%\n' "$engine_pid_count" "$engine_cpu"
    printf '  Engine RAM       : %-18s Connections : %s\n' "$(format_bytes "$engine_rss")" "$connections"
    printf '  Established      : %-18s Listening   : %s\n' "$established" "$listening"
    printf '  RTT              : %-18s Jitter      : %s\n' "$([[ "$rtt" == N/A ]] && echo N/A || echo "$rtt ms")" "$([[ "$jitter" == N/A ]] && echo N/A || echo "$jitter ms")"
    printf '  Packet Loss      : %s\n' "$([[ "$loss" == N/A ]] && echo N/A || echo "$loss%")"

    printf '\n  [L] Logs   [D] Diagnose   [R] Restart   [S] Snapshot   [Q] Quit\n'
    read -r -t 1 -n 1 key || key=""
    case "${key,,}" in
      q) break;;
      l) view_service_logs "$service";;
      d) diagnostic_center;;
      r) systemctl restart "$service";;
      s) network_snapshot_create;;
    esac
  done
}

tunnel_stress_test() {
  ensure_iperf3 || { press_key; return; }; mkdir -p "$STRESS_DIR"; clear; section_header "Tunnel Stress Test"
  local host port minutes streams direction extra=() stamp raw summary; read -r -p "Receiver IP or hostname: " host; [[ -n "$host" ]] || return; valid_port_prompt "Receiver port" "5201" port; prompt_positive_int "Duration minutes" "5" minutes; prompt_positive_int "Parallel streams" "4" streams
  select_option "Direction" "1" direction "Upload" "Download (reverse)"; [[ "$direction" == "Download (reverse)" ]] && extra+=(--reverse); stamp=$(date +%Y%m%d-%H%M%S); raw="$STRESS_DIR/stress-$stamp.log"; summary="$STRESS_DIR/stress-$stamp.txt"
  clear; section_header "Live Stress Test"; iperf3 -c "$host" -p "$port" -t "$((minutes*60))" -P "$streams" -i 1 --forceflush "${extra[@]}" 2>&1 | tee "$raw"; local rc=${PIPESTATUS[0]}; ((rc==0)) || { colorize red "Stress test failed. Log: $raw"; press_key; return; }
  python3 - "$raw" "$summary" <<'PYEND'
import re,sys,statistics,datetime
raw,out=sys.argv[1:]; vals=[]
for line in open(raw,errors='ignore'):
 m=re.search(r'([0-9.]+)\s+([KMG])bits/sec',line)
 if m and ('[SUM]' in line or 'receiver' in line): vals.append(float(m.group(1))*{'K':.001,'M':1,'G':1000}[m.group(2)])
if vals:
 avg=statistics.mean(vals); stability=max(0,100-(statistics.pstdev(vals)/avg*100 if avg else 100)); text=f"TixoTunnel Stress Test\nGenerated: {datetime.datetime.now().isoformat(timespec='seconds')}\nSamples: {len(vals)}\nAverage: {avg:.2f} Mbit/s\nPeak: {max(vals):.2f} Mbit/s\nMinimum: {min(vals):.2f} Mbit/s\nStability Score: {stability:.1f}/100\n"
else: text='Unable to parse stress-test output.\n'
open(out,'w').write(text); print('\n'+text)
PYEND
  printf "  Summary: %s\n  Raw log: %s\n" "$summary" "$raw"; press_key
}
backup_create() {
  mkdir -p "$BACKUP_DIR"; local file manifest; file="$BACKUP_DIR/tixotunnel-backup-$(date +%Y%m%d-%H%M%S).tar.gz"; manifest=$(mktemp); find "$config_dir" -maxdepth 2 -type f ! -path "$BACKUP_DIR/*" -print >"$manifest" 2>/dev/null || true; find /etc/systemd/system -maxdepth 1 -type f \( -name 'tixotunnel-*.service' -o -name 'tixotunnel-*.timer' \) -print >>"$manifest" 2>/dev/null || true; [[ -f "$PANEL_PATH" ]] && echo "$PANEL_PATH" >>"$manifest"; tar -czf "$file" --absolute-names --files-from "$manifest" 2>/dev/null; rm -f "$manifest"; colorize green "Backup created." bold; printf "\n  %s\n\n" "$file"; press_key
}
backup_restore() {
  mkdir -p "$BACKUP_DIR"; local file choice; file=$(ls -1t "$BACKUP_DIR"/tixotunnel-backup-*.tar.gz 2>/dev/null | head -1); [[ -f "$file" ]] || { colorize yellow "No backup found."; press_key; return; }; printf "Latest backup: %s\n" "$file"; read -r -p "Type RESTORE to continue: " choice; [[ "$choice" == RESTORE ]] || return; tar -xzf "$file" -C /; systemctl daemon-reload; colorize green "Backup restored." bold; press_key
}
backup_center() { clear; section_header "Backup & Recovery"; echo "  [1] Create Backup"; echo "  [2] Restore Latest Backup"; echo "  [3] List Backups"; echo "  [0] Back"; echo; read -r -p "Select: " b; case "$b" in 1) backup_create;; 2) backup_restore;; 3) ls -lh "$BACKUP_DIR"/*.tar.gz 2>/dev/null || echo 'No backups.'; press_key;; esac; }
export_diagnostic_bundle() {
  mkdir -p "$DIAG_DIR"; local stamp dir archive service; stamp=$(date +%Y%m%d-%H%M%S); dir="$DIAG_DIR/bundle-$stamp"; archive="$DIAG_DIR/tixotunnel-support-$stamp.tar.gz"; mkdir -p "$dir"; network_snapshot_create_silent "$dir/network.txt"; service=$(first_tunnel_service); [[ -n "$service" ]] && journalctl -u "$service" --since '24 hours ago' --no-pager -o short-iso >"$dir/service.log" 2>/dev/null || true; cp -a "$config_dir"/*.toml "$dir"/ 2>/dev/null || true; systemctl --no-pager list-units 'tixotunnel-*' >"$dir/services.txt" 2>/dev/null || true; tar -czf "$archive" -C "$DIAG_DIR" "bundle-$stamp"; rm -rf "$dir"; clear; section_header "Diagnostic Export"; colorize green "Support bundle created." bold; printf "\n  %s\n\n" "$archive"; press_key
}
operations_center() {
  clear; section_header "Operations & Diagnostics"; echo "  [1] Tunnel Diagnostic Center"; echo "  [2] Live Dashboard"; echo "  [3] Log Analyzer"; echo "  [4] Network Snapshot"; echo "  [5] Tunnel Stress Test"; echo "  [6] Backup & Recovery"; echo "  [7] Export Support Bundle"; echo "  [0] Back"; echo; read -r -p "Select: " o
  case "$o" in 1) diagnostic_center;; 2) live_dashboard;; 3) log_analyzer;; 4) network_snapshot_create;; 5) tunnel_stress_test;; 6) backup_center;; 7) export_diagnostic_bundle;; esac
}

complete_uninstall() {
  clear; section_header "Complete Uninstall"
  colorize red "This removes every TixoTunnel service, config, scheduler, binary and panel command." bold
  read -r -p "Type REMOVE to continue: " confirm; [[ "$confirm" == REMOVE ]] || return
  for unit in /etc/systemd/system/tixotunnel-*.service /etc/systemd/system/tixotunnel-*.timer; do [[ -e "$unit" ]] || continue; systemctl disable --now "$(basename "$unit")" >/dev/null 2>&1 || true; rm -f "$unit"; done
  systemctl daemon-reload
  rm -rf "$config_dir" /root/tixotunnel-core
  rm -f /usr/local/bin/tixotunnel
  clear
  echo; colorize cyan "Thank you for using TixoTunnel." bold
  echo "All services and files have been removed safely. Goodbye 👋"; echo
  local self; self=$(readlink -f "$0" 2>/dev/null || echo "$0")
  [[ "$self" == /root/TixoTunnel.sh || "$self" == /usr/local/bin/tixotunnel ]] && rm -f "$self"
  rm -f /root/TixoTunnel.sh
  exit 0
}

tunnel_management() {
  if ! ls "$config_dir"/*.toml >/dev/null 2>&1; then colorize red "No config files found." bold; press_key; return; fi
  clear; section_header "Tunnel Manager"
  local index=1 config_path config_name service_name status note; declare -a configs
  for config_path in "$config_dir"/{iran,kharej}*.toml; do [[ -f "$config_path" ]] || continue; configs+=("$config_path"); config_name=$(basename "${config_path%.toml}"); service_name="tixotunnel-${config_name}.service"; systemctl is-active --quiet "$service_name" && status="\033[38;5;46m● Running\033[0m" || status="\033[38;5;245m○ Stopped\033[0m"; note=""; [[ -s "${config_path%.toml}.notes" ]] && note=" · 📝"; echo -e "  \033[38;5;51m[$index]\033[0m $config_name · $status$note"; ((index++)); done
  echo; read -r -p "Select tunnel [0 = back]: " choice; [[ "$choice" == 0 ]] && return
  [[ "$choice" =~ ^[0-9]+$ ]] && ((choice>=1 && choice<=${#configs[@]})) || return
  selected_config=${configs[$((choice-1))]}; config_name=$(basename "${selected_config%.toml}"); service_name="tixotunnel-${config_name}.service"
  clear; section_header "Tunnel Control — $config_name"
  echo "  [1] Tunnel Health"; echo "  [2] Restart Tunnel"; echo "  [3] Live Log Filter"; echo "  [4] Service Details"; echo "  [5] Restart Scheduler"; echo "  [6] Tunnel Notes"; echo "  [7] Remove Tunnel"; echo "  [0] Back"; echo
  read -r -p "Select: " action
  case "$action" in 1) tunnel_health "$selected_config";; 2) restart_service "$service_name";; 3) view_service_logs "$service_name";; 4) view_service_status "$service_name";; 5) restart_scheduler "$service_name";; 6) tunnel_notes "$selected_config";; 7) destroy_tunnel "$selected_config";; esac
}


network_optimization_menu() {
  while true; do
    clear
    section_header "Network Optimization"
    echo "  [1] Enable TCP BBR"
    echo "  [2] Check TCP Optimization Status"
    echo "  [3] Apply Kernel Performance Profile"
    echo "  [4] Disable BBR"
    echo "  [0] Back"
    echo
    read -r -p "Select: " n
    case "$n" in
      1) enable_bbr ;;
      2) check_bbr_status ;;
      3) apply_network_profile ;;
      4) disable_bbr ;;
      0) return ;;
    esac
  done
}

enable_bbr() {
  clear
  section_header "Enable TCP BBR"
  if ! modprobe tcp_bbr 2>/dev/null; then
    colorize red "Kernel does not support TCP BBR."
    press_key
    return
  fi
  cat >/etc/sysctl.d/99-tixotunnel-bbr.conf <<EOF
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF
  sysctl --system >/dev/null 2>&1 || true
  colorize green "TCP BBR enabled successfully."
  check_bbr_status
  press_key
}

check_bbr_status() {
  echo
  echo "TCP Stack Status"
  echo "----------------"
  printf "  Queue        : "
  sysctl -n net.core.default_qdisc 2>/dev/null || echo unknown
  printf "  Congestion   : "
  sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null || echo unknown
}

disable_bbr() {
  rm -f /etc/sysctl.d/99-tixotunnel-bbr.conf
  sysctl --system >/dev/null 2>&1 || true
  colorize green "BBR configuration removed."
  press_key
}

apply_network_profile() {
  enable_bbr
}

display_menu() {
  display_logo; display_server_info; display_engine_status; echo
  colorize green   " [1] Create Tunnel" bold
  colorize cyan    " [2] Tunnel Manager" bold
  colorize yellow  " [3] Operations & Diagnostics" bold
  colorize magenta " [4] Advanced Spoof Tester" bold
  echo              " [5] Connection Benchmark"
  echo              " [6] Link Benchmark Pro"
  echo              " [7] Server Speed Test"
  echo              " [8] Network Information"
  echo              " [9] Network Optimization"
  echo              " [10] Update Center"
  colorize red      "[11] Complete Uninstall" bold
  echo              " [0] Exit"
  echo -e "\033[38;5;245m────────────────────────────────────────────────────────────────────\033[0m"
}
update_center() {
  clear; section_header "Update Center"
  echo "  [1] Check for Updates"
  echo "  [2] Update Everything"
  echo "  [3] Roll Back Last Unified Update"
  echo "  [0] Back"
  echo
  read -r -p "Select: " u
  case "$u" in 1) check_updates;; 2) unified_update;; 3) rollback_last_update;; esac
}
read_option() {
  read -r -p "Select an option [0-11]: " choice
  case "$choice" in 1) configure_tunnel;; 2) tunnel_management;; 3) operations_center;; 4) spoof_tester_run;; 5) connection_benchmark;; 6) link_benchmark_menu;; 7) server_speedtest_menu;; 8) network_information;; 9) network_optimization_menu;; 10) update_center;; 11) complete_uninstall;; 0) clear; exit 0;; *) colorize red "Invalid option."; sleep 1;; esac
}
while true; do display_menu; read_option; done
