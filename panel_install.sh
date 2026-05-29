#!/bin/bash
set -e

# 瑙ｅ喅 macOS 涓?tr 鍙兘鍑虹幇鐨勯潪娉曞瓧鑺傚簭鍒楅棶棰?
export LANG=en_US.UTF-8
export LC_ALL=C



# GitHub repo used for release downloads
REPO="iKeilo/flvxt2"

# 鍥哄畾鐗堟湰鍙凤紙Release 鏋勫缓鏃惰嚜鍔ㄥ～鍏咃紝鐣欑┖鍒欒幏鍙栨渶鏂扮増锛?
PINNED_VERSION=""

# 闀滃儚鍔犻€熼厤缃紙鍙敱闈㈡澘浼犲叆鎴栦氦浜掑紡璇㈤棶锛?
PROXY_ENABLED="${PROXY_ENABLED:-}"
PROXY_URL="${PROXY_URL:-}"

# 闀滃儚鍔犻€?
maybe_proxy_url() {
  local url="$1"

  if [[ "$PROXY_ENABLED" == "false" ]]; then
    echo "$url"
    return
  fi

  local proxy="${PROXY_URL:-gcode.hostcentral.cc}"

  if [[ "$proxy" == https://* || "$proxy" == http://* ]]; then
    proxy="${proxy%/}"
  else
    proxy="https://${proxy%/}"
  fi

  echo "${proxy}/${url}"
}

ask_proxy_config() {
  if [[ -n "$PROXY_ENABLED" ]]; then
    return
  fi

  if [[ -n "$PROXY_URL" ]]; then
    PROXY_ENABLED="true"
    return
  fi

  echo ""
  echo "==============================================="
  echo "           GitHub 鍔犻€熼厤缃?
  echo "==============================================="
  if ! read -r -p "鏄惁寮€鍚?GitHub 鍔犻€? (Y/n): " proxy_choice; then
    proxy_choice=""
  fi
  case "$proxy_choice" in
    n|N)
      PROXY_ENABLED="false"
      echo "宸插叧闂姞閫燂紝灏嗙洿杩?GitHub"
      ;;
    *)
      PROXY_ENABLED="true"
      if ! read -r -p "鍔犻€熷湴鍧€ (榛樿 gcode.hostcentral.cc): " input_url; then
        input_url=""
      fi
      PROXY_URL="${input_url:-gcode.hostcentral.cc}"
      echo "宸插紑鍚姞閫? $PROXY_URL"
      ;;
  esac
  echo "==============================================="
}

resolve_latest_release_tag() {
  local effective_url tag api_tag latest_url api_url

  latest_url="https://github.com/${REPO}/releases/latest"
  api_url="https://api.github.com/repos/${REPO}/releases/latest"

  effective_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' -L "$(maybe_proxy_url "$latest_url")" 2>/dev/null || true)
  tag="${effective_url##*/}"
  if [[ -n "$tag" && "$tag" != "latest" ]]; then
    echo "$tag"
    return 0
  fi

  api_tag=$(curl -fsSL "$(maybe_proxy_url "$api_url")" 2>/dev/null | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' || true)
  if [[ -n "$api_tag" ]]; then
    echo "$api_tag"
    return 0
  fi

  return 1
}

resolve_version() {
  if [[ -n "${VERSION:-}" ]]; then
    echo "$VERSION"
    return 0
  fi
  if [[ -n "${FLUX_VERSION:-}" ]]; then
    echo "$FLUX_VERSION"
    return 0
  fi
  if [[ -n "${PINNED_VERSION:-}" ]]; then
    echo "$PINNED_VERSION"
    return 0
  fi

  if resolve_latest_release_tag; then
    return 0
  fi

  echo "鉂?鏃犳硶鑾峰彇鏈€鏂扮増鏈彿銆備綘鍙互鎵嬪姩鎸囧畾鐗堟湰锛屼緥濡傦細VERSION=<鐗堟湰鍙? ./panel_install.sh" >&2
  return 1
}

normalize_image_version() {
  local version="$1"
  version="${version#Release}"
  echo "$version"
}


# 鏍规嵁鐗堟湰鍙疯缃?compose 涓嬭浇鍦板潃
set_compose_urls_by_version() {
  local version="$1"
  DOCKER_COMPOSEV4_URL=$(maybe_proxy_url "https://github.com/${REPO}/releases/download/${version}/docker-compose-v4.yml")
  DOCKER_COMPOSEV6_URL=$(maybe_proxy_url "https://github.com/${REPO}/releases/download/${version}/docker-compose-v6.yml")
}

ensure_compose_urls_initialized() {
  if [[ -n "${DOCKER_COMPOSEV4_URL:-}" && -n "${DOCKER_COMPOSEV6_URL:-}" ]]; then
    return 0
  fi

  RESOLVED_VERSION=$(resolve_version) || return 1
  set_compose_urls_by_version "$RESOLVED_VERSION"
}



# 鏍规嵁IPv6鏀寔鎯呭喌閫夋嫨docker-compose URL
get_docker_compose_url() {
  if check_ipv6_support > /dev/null 2>&1; then
    echo "$DOCKER_COMPOSEV6_URL"
  else
    echo "$DOCKER_COMPOSEV4_URL"
  fi
}

# 妫€鏌?docker-compose 鎴?docker compose 鍛戒护
check_docker() {
  if command -v docker-compose &> /dev/null; then
    DOCKER_CMD="docker-compose"
  elif command -v docker &> /dev/null; then
    if docker compose version &> /dev/null; then
      DOCKER_CMD="docker compose"
    else
      echo "閿欒锛氭娴嬪埌 docker锛屼絾涓嶆敮鎸?'docker compose' 鍛戒护銆傝瀹夎 docker-compose 鎴栨洿鏂?docker 鐗堟湰銆?
      exit 1
    fi
  else
    echo "閿欒锛氭湭妫€娴嬪埌 docker 鎴?docker-compose 鍛戒护銆傝鍏堝畨瑁?Docker銆?
    exit 1
  fi
  echo "妫€娴嬪埌 Docker 鍛戒护锛?DOCKER_CMD"
}

# 妫€娴嬬郴缁熸槸鍚︽敮鎸?IPv6
check_ipv6_support() {
  echo "馃攳 妫€娴?IPv6 鏀寔..."

  # 妫€鏌ユ槸鍚︽湁 IPv6 鍦板潃锛堟帓闄?link-local 鍦板潃锛?
  if ip -6 addr show | grep -v "scope link" | grep -q "inet6"; then
    echo "鉁?妫€娴嬪埌绯荤粺鏀寔 IPv6"
    return 0
  elif ifconfig 2>/dev/null | grep -v "fe80:" | grep -q "inet6"; then
    echo "鉁?妫€娴嬪埌绯荤粺鏀寔 IPv6"
    return 0
  else
    echo "鈿狅笍 鏈娴嬪埌 IPv6 鏀寔"
    return 1
  fi
}



# 閰嶇疆 Docker 鍚敤 IPv6
configure_docker_ipv6() {
  echo "馃敡 閰嶇疆 Docker IPv6 鏀寔..."

  # 妫€鏌ユ搷浣滅郴缁熺被鍨?
  OS_TYPE=$(uname -s)

  if [[ "$OS_TYPE" == "Darwin" ]]; then
    # macOS 涓?Docker Desktop 宸查粯璁ゆ敮鎸?IPv6
    echo "鉁?macOS Docker Desktop 榛樿鏀寔 IPv6"
    return 0
  fi

  # Docker daemon 閰嶇疆鏂囦欢璺緞
  DOCKER_CONFIG="/etc/docker/daemon.json"

  # 妫€鏌ユ槸鍚﹂渶瑕?sudo
  if [[ $EUID -ne 0 ]]; then
    SUDO_CMD="sudo"
  else
    SUDO_CMD=""
  fi

  # 妫€鏌?Docker 閰嶇疆鏂囦欢
  if [ -f "$DOCKER_CONFIG" ]; then
    # 妫€鏌ユ槸鍚﹀凡缁忛厤缃簡 IPv6
    if grep -q '"ipv6"' "$DOCKER_CONFIG"; then
      echo "鉁?Docker 宸查厤缃?IPv6 鏀寔"
    else
      echo "馃摑 鏇存柊 Docker 閰嶇疆浠ュ惎鐢?IPv6..."
      # 澶囦唤鍘熼厤缃?
      $SUDO_CMD cp "$DOCKER_CONFIG" "${DOCKER_CONFIG}.backup"

      # 浣跨敤 jq 鎴?sed 娣诲姞 IPv6 閰嶇疆
      if command -v jq &> /dev/null; then
        $SUDO_CMD jq '. + {"ipv6": true, "fixed-cidr-v6": "fd00::/80"}' "$DOCKER_CONFIG" > /tmp/daemon.json && $SUDO_CMD mv /tmp/daemon.json "$DOCKER_CONFIG"
      else
        # 濡傛灉娌℃湁 jq锛屼娇鐢?sed
        $SUDO_CMD sed -i 's/^{$/{\n  "ipv6": true,\n  "fixed-cidr-v6": "fd00::\/80",/' "$DOCKER_CONFIG"
      fi

      echo "馃攧 閲嶅惎 Docker 鏈嶅姟..."
      if command -v systemctl &> /dev/null; then
        $SUDO_CMD systemctl restart docker
      elif command -v service &> /dev/null; then
        $SUDO_CMD service docker restart
      else
        echo "鈿狅笍 璇锋墜鍔ㄩ噸鍚?Docker 鏈嶅姟"
      fi
      sleep 5
    fi
  else
    # 鍒涘缓鏂扮殑閰嶇疆鏂囦欢
    echo "馃摑 鍒涘缓 Docker 閰嶇疆鏂囦欢..."
    $SUDO_CMD mkdir -p /etc/docker
    echo '{
  "ipv6": true,
  "fixed-cidr-v6": "fd00::/80"
}' | $SUDO_CMD tee "$DOCKER_CONFIG" > /dev/null

    echo "馃攧 閲嶅惎 Docker 鏈嶅姟..."
    if command -v systemctl &> /dev/null; then
      $SUDO_CMD systemctl restart docker
    elif command -v service &> /dev/null; then
      $SUDO_CMD service docker restart
    else
      echo "鈿狅笍 璇锋墜鍔ㄩ噸鍚?Docker 鏈嶅姟"
    fi
    sleep 5
  fi
}

# 鏄剧ず鑿滃崟
show_menu() {
  echo "==============================================="
  echo "          闈㈡澘绠＄悊鑴氭湰"
  echo "==============================================="
  echo "璇烽€夋嫨鎿嶄綔锛?
  echo "1. 瀹夎闈㈡澘"
  echo "2. 鏇存柊闈㈡澘"
  echo "3. 鍗歌浇闈㈡澘"
  echo "4. 杩佺Щ鍒?PostgreSQL"
  echo "5. 閫€鍑?
  echo "==============================================="
}

generate_random() {
  LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c16
}

upsert_env_var() {
  local file="$1"
  local key="$2"
  local value="$3"
  local tmp_file

  tmp_file=$(mktemp)
  if [ -f "$file" ]; then
    awk -v k="$key" -v v="$value" '
      BEGIN { found=0 }
      $0 ~ ("^" k "=") { print k "=" v; found=1; next }
      { print }
      END { if (!found) print k "=" v }
    ' "$file" > "$tmp_file"
  else
    printf '%s=%s\n' "$key" "$value" > "$tmp_file"
  fi

  mv "$tmp_file" "$file"
}

get_env_var() {
  local key="$1"
  local file="${2:-.env}"

  if [[ ! -f "$file" ]]; then
    return 0
  fi

  grep -m1 "^${key}=" "$file" | cut -d= -f2-
}

get_current_db_type() {
  local db_type database_url

  db_type=$(get_env_var "DB_TYPE")
  database_url=$(get_env_var "DATABASE_URL")

  if [[ "$db_type" == "sqlite" ]]; then
    echo "sqlite"
  elif [[ "$db_type" == "postgres" || "$database_url" == postgres://* || "$database_url" == postgresql://* ]]; then
    echo "postgres"
  else
    echo "sqlite"
  fi
}

wait_for_postgres_healthy() {
  local pg_health

  echo "馃攳 妫€鏌?PostgreSQL 鏈嶅姟鐘舵€?.."
  for i in {1..90}; do
    if docker ps --format "{{.Names}}" | grep -q "^flvx-svc-postgres$"; then
      pg_health=$(docker inspect -f '{{.State.Health.Status}}' flvx-svc-postgres 2>/dev/null || echo "unknown")
      if [[ "$pg_health" == "healthy" ]]; then
        echo "鉁?PostgreSQL 鏈嶅姟鍋ュ悍妫€鏌ラ€氳繃"
        return 0
      elif [[ "$pg_health" == "unhealthy" ]]; then
        echo "鈿狅笍 PostgreSQL 鍋ュ悍鐘舵€侊細$pg_health"
      fi
    else
      pg_health="not_running"
    fi

    if [ $i -eq 90 ]; then
      echo "鉂?PostgreSQL 鍚姩瓒呮椂锛?0绉掞級"
      echo "馃攳 褰撳墠鐘舵€侊細$(docker inspect -f '{{.State.Health.Status}}' flvx-svc-postgres 2>/dev/null || echo '瀹瑰櫒涓嶅瓨鍦?)"
      return 1
    fi

    if [ $((i % 15)) -eq 1 ]; then
      echo "鈴?绛夊緟 PostgreSQL 鍚姩... ($i/90) 鐘舵€侊細${pg_health:-unknown}"
    fi
    sleep 1
  done
}

wait_for_backend_healthy() {
  local backend_health

  echo "馃攳 妫€鏌ュ悗绔湇鍔＄姸鎬?.."
  for i in {1..90}; do
    if docker ps --format "{{.Names}}" | grep -q "^flvx-svc-backend$"; then
      backend_health=$(docker inspect -f '{{.State.Health.Status}}' flvx-svc-backend 2>/dev/null || echo "unknown")
      if [[ "$backend_health" == "healthy" ]]; then
        echo "鉁?鍚庣鏈嶅姟鍋ュ悍妫€鏌ラ€氳繃"
        return 0
      elif [[ "$backend_health" == "unhealthy" ]]; then
        echo "鈿狅笍 鍚庣鍋ュ悍鐘舵€侊細$backend_health"
      fi
    else
      backend_health="not_running"
    fi

    if [ $i -eq 90 ]; then
      echo "鉂?鍚庣鏈嶅姟鍚姩瓒呮椂锛?0绉掞級"
      echo "馃攳 褰撳墠鐘舵€侊細$(docker inspect -f '{{.State.Health.Status}}' flvx-svc-backend 2>/dev/null || echo '瀹瑰櫒涓嶅瓨鍦?)"
      return 1
    fi

    if [ $((i % 15)) -eq 1 ]; then
      echo "鈴?绛夊緟鍚庣鏈嶅姟鍚姩... ($i/90) 鐘舵€侊細${backend_health:-unknown}"
    fi
    sleep 1
  done
}

# 鍒犻櫎鑴氭湰鑷韩
delete_self() {
  echo ""
  echo "馃棏锔?鎿嶄綔宸插畬鎴愶紝姝ｅ湪娓呯悊鑴氭湰鏂囦欢..."
  SCRIPT_PATH="$(readlink -f "$0" 2>/dev/null || realpath "$0" 2>/dev/null || echo "$0")"
  sleep 1
  rm -f "$SCRIPT_PATH" && echo "鉁?鑴氭湰鏂囦欢宸插垹闄? || echo "鉂?鍒犻櫎鑴氭湰鏂囦欢澶辫触"
}



# 鑾峰彇鐢ㄦ埛杈撳叆鐨勯厤缃弬鏁?
get_config_params() {
  echo "馃敡 璇疯緭鍏ラ厤缃弬鏁帮細"

  read -p "鍓嶇绔彛锛堥粯璁?6366锛? " FRONTEND_PORT
  FRONTEND_PORT=${FRONTEND_PORT:-6366}

  read -p "鍚庣绔彛锛堥粯璁?6365锛? " BACKEND_PORT
  BACKEND_PORT=${BACKEND_PORT:-6365}

  echo "璇烽€夋嫨鏁版嵁搴撶被鍨嬶細"
  echo "1. SQLite锛堥粯璁わ級"
  echo "2. PostgreSQL"
  read -p "鏁版嵁搴撶被鍨嬶紙1/2锛岄粯璁?1锛? " DB_CHOICE
  case "$DB_CHOICE" in
    2)
      DB_TYPE="postgres"
      ;;
    ""|1)
      DB_TYPE="sqlite"
      ;;
    *)
      echo "鈿狅笍 杈撳叆鏃犳晥锛岄粯璁や娇鐢?SQLite"
      DB_TYPE="sqlite"
      ;;
  esac

  POSTGRES_DB="flvx_svc"
  POSTGRES_USER="flvx_svc"
  POSTGRES_PASSWORD=$(generate_random)

  if [[ "$DB_TYPE" == "postgres" ]]; then
    DATABASE_URL="postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable"
  else
    DATABASE_URL=""
  fi

  # 鐢熸垚JWT瀵嗛挜
  JWT_SECRET=$(generate_random)
}

# 瀹夎鍔熻兘
install_panel() {
  echo "馃殌 寮€濮嬪畨瑁呴潰鏉?.."

  ask_proxy_config
  ensure_compose_urls_initialized || return 1

  check_docker
  get_config_params

  echo "馃斀 涓嬭浇蹇呰鏂囦欢..."
  DOCKER_COMPOSE_URL=$(get_docker_compose_url)
  echo "馃摗 閫夋嫨閰嶇疆鏂囦欢锛?(basename "$DOCKER_COMPOSE_URL")"
  curl -L -o docker-compose.yml "$DOCKER_COMPOSE_URL"
  echo "鉁?鏂囦欢鍑嗗瀹屾垚"

  # 鑷姩妫€娴嬪苟閰嶇疆 IPv6 鏀寔
  if check_ipv6_support; then
    echo "馃殌 绯荤粺鏀寔 IPv6锛岃嚜鍔ㄥ惎鐢?IPv6 閰嶇疆..."
    configure_docker_ipv6
  fi

  cat > .env <<EOF
JWT_SECRET=$JWT_SECRET
FRONTEND_PORT=$FRONTEND_PORT
BACKEND_PORT=$BACKEND_PORT
FLUX_VERSION=$RESOLVED_VERSION
IMAGE_VERSION=$(normalize_image_version "$RESOLVED_VERSION")

DB_TYPE=$DB_TYPE
DATABASE_URL=$DATABASE_URL

POSTGRES_DB=$POSTGRES_DB
POSTGRES_USER=$POSTGRES_USER
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
EOF

  echo "馃殌 鍚姩 docker 鏈嶅姟..."
  if [[ "$DB_TYPE" == "postgres" ]]; then
    $DOCKER_CMD up -d postgres
    wait_for_postgres_healthy
    $DOCKER_CMD up -d backend frontend
  else
    $DOCKER_CMD up -d backend frontend
  fi

  echo "馃帀 閮ㄧ讲瀹屾垚"
  echo "馃寪 璁块棶鍦板潃: http://鏈嶅姟鍣↖P:$FRONTEND_PORT"
  echo "馃摉 閮ㄧ讲瀹屾垚鍚庤闃呰涓嬩娇鐢ㄦ枃妗ｏ紝姹傛眰浜嗗晩锛屼笉瑕佷笂鍘诲氨鏄竴椤挎搷浣?
  echo "馃摎 鏂囨。鍦板潃: https://tes.cc/guide.html"
  echo "馃挕 榛樿绠＄悊鍛樿处鍙? admin_user / admin_user"
  echo "鈿狅笍  鐧诲綍鍚庤绔嬪嵆淇敼榛樿瀵嗙爜锛?


}

# 鏇存柊鍔熻兘
update_panel() {
  echo "馃攧 寮€濮嬫洿鏂伴潰鏉?.."
  ask_proxy_config
  check_docker

  if [[ ! -f ".env" ]]; then
    echo "鈿狅笍 鏈壘鍒?.env锛岄粯璁ゆ寜 SQLite 妯″紡鏇存柊"
  fi
  CURRENT_DB_TYPE=$(get_current_db_type)
  echo "馃梽锔?褰撳墠鏁版嵁搴撶被鍨嬶細$CURRENT_DB_TYPE"

  echo "馃攳 鑾峰彇鏈€鏂扮増鏈彿..."
  LATEST_VERSION=$(resolve_latest_release_tag) || {
    echo "鉂?鏃犳硶鑾峰彇鏈€鏂扮増鏈彿锛屾洿鏂扮粓姝?
    return 1
  }
  echo "馃啎 鏈€鏂扮増鏈細$LATEST_VERSION"
  set_compose_urls_by_version "$LATEST_VERSION"
  upsert_env_var ".env" "FLUX_VERSION" "$LATEST_VERSION"
  upsert_env_var ".env" "IMAGE_VERSION" "$(normalize_image_version "$LATEST_VERSION")"

  echo "馃斀 涓嬭浇鏈€鏂伴厤缃枃浠?.."
  DOCKER_COMPOSE_URL=$(get_docker_compose_url)
  echo "馃摗 閫夋嫨閰嶇疆鏂囦欢锛?(basename "$DOCKER_COMPOSE_URL")"
  curl -L -o docker-compose.yml "$DOCKER_COMPOSE_URL"
  echo "鉁?涓嬭浇瀹屾垚"

  # 鑷姩妫€娴嬪苟閰嶇疆 IPv6 鏀寔
  if check_ipv6_support; then
    echo "馃殌 绯荤粺鏀寔 IPv6锛岃嚜鍔ㄥ惎鐢?IPv6 閰嶇疆..."
    configure_docker_ipv6
  fi

  # 鍏堝彂閫?SIGTERM 淇″彿锛岃搴旂敤浼橀泤鍏抽棴
  docker stop -t 30 flvx-svc-backend 2>/dev/null || true
  docker stop -t 10 flvx-svc-frontend 2>/dev/null || true
  
  # 绛夊緟 WAL 鏂囦欢鍚屾
  echo "鈴?绛夊緟鏁版嵁鍚屾..."
  sleep 5
  
  # 鐒跺悗鍐嶅畬鍏ㄥ仠姝?
  $DOCKER_CMD down

  echo "猬囷笍 鎷夊彇鏈€鏂伴暅鍍?.."
  if [[ "$CURRENT_DB_TYPE" == "postgres" ]]; then
    $DOCKER_CMD pull backend frontend postgres
  else
    $DOCKER_CMD pull backend frontend
  fi

  echo "馃殌 鍚姩鏇存柊鍚庣殑鏈嶅姟..."
  if [[ "$CURRENT_DB_TYPE" == "postgres" ]]; then
    $DOCKER_CMD up -d postgres
    wait_for_postgres_healthy
    $DOCKER_CMD up -d backend frontend
  else
    $DOCKER_CMD up -d backend frontend
  fi

  # 绛夊緟鏈嶅姟鍚姩
  echo "鈴?绛夊緟鏈嶅姟鍚姩..."

  if ! wait_for_backend_healthy; then
    echo "馃洃 鏇存柊缁堟"
    return 1
  fi

  echo "鉁?鏇存柊瀹屾垚"
}


migrate_to_postgres() {
  local current_db_type postgres_db postgres_user postgres_password database_url

  echo "馃攧 寮€濮嬭縼绉?SQLite -> PostgreSQL..."
  check_docker

  if [[ ! -f ".env" ]]; then
    echo "鉂?鏈壘鍒?.env 鏂囦欢锛岃鍏堝畨瑁呴潰鏉?
    return 1
  fi

  if [[ ! -f "docker-compose.yml" ]]; then
    echo "鈿狅笍 鏈壘鍒?docker-compose.yml 鏂囦欢锛屾鍦ㄤ笅杞?.."
    ask_proxy_config
    ensure_compose_urls_initialized || return 1
    DOCKER_COMPOSE_URL=$(get_docker_compose_url)
    echo "馃摗 閫夋嫨閰嶇疆鏂囦欢锛?(basename "$DOCKER_COMPOSE_URL")"
    curl -L -o docker-compose.yml "$DOCKER_COMPOSE_URL"
    echo "鉁?docker-compose.yml 涓嬭浇瀹屾垚"
  fi

  current_db_type=$(get_current_db_type)
  if [[ "$current_db_type" == "postgres" ]]; then
    echo "鈩癸笍 褰撳墠宸蹭娇鐢?PostgreSQL锛屾棤闇€杩佺Щ"
    return 0
  fi

  postgres_db=$(get_env_var "POSTGRES_DB")
  postgres_user=$(get_env_var "POSTGRES_USER")
  postgres_password=$(get_env_var "POSTGRES_PASSWORD")

  postgres_db=${postgres_db:-flvx_svc}
  postgres_user=${postgres_user:-flvx_svc}
  postgres_password=${postgres_password:-$(generate_random)}

  upsert_env_var ".env" "POSTGRES_DB" "$postgres_db"
  upsert_env_var ".env" "POSTGRES_USER" "$postgres_user"
  upsert_env_var ".env" "POSTGRES_PASSWORD" "$postgres_password"

  echo "馃洃 鍋滄褰撳墠鏈嶅姟..."
  docker stop -t 30 flvx-svc-backend 2>/dev/null || true
  docker stop -t 10 flvx-svc-frontend 2>/dev/null || true
  echo "鈴?绛夊緟鏁版嵁鍚屾..."
  sleep 5
  $DOCKER_CMD down

  echo "馃捑 澶囦唤 SQLite 鏁版嵁鍒板綋鍓嶇洰褰?.."
  if ! docker run --rm -v sqlite_data:/data -v "$(pwd)":/backup alpine sh -c "cp /data/gost.db /backup/gost.db.bak"; then
    echo "鉂?SQLite 澶囦唤澶辫触锛岃縼绉荤粓姝?
    return 1
  fi

  echo "馃殌 鍚姩 PostgreSQL..."
  $DOCKER_CMD up -d postgres
  if ! wait_for_postgres_healthy; then
    echo "馃洃 PostgreSQL 鏈氨缁紝杩佺Щ缁堟"
    return 1
  fi

  echo "馃攧 鎵ц pgloader 杩佺Щ..."
  if ! docker run --rm --network gost-network -v sqlite_data:/sqlite dimitri/pgloader:latest pgloader /sqlite/gost.db "postgresql://${postgres_user}:${postgres_password}@postgres:5432/${postgres_db}"; then
    echo "鉂?pgloader 杩佺Щ澶辫触锛岃縼绉荤粓姝紙濡傛姤 28P01锛屽彲鎵ц docker volume rm postgres_data 鍚庨噸璇曪級"
    return 1
  fi

  database_url="postgresql://${postgres_user}:${postgres_password}@postgres:5432/${postgres_db}?sslmode=disable"
  upsert_env_var ".env" "DB_TYPE" "postgres"
  upsert_env_var ".env" "DATABASE_URL" "$database_url"

  echo "馃殌 鍚姩杩佺Щ鍚庣殑鏈嶅姟..."
  $DOCKER_CMD up -d postgres backend frontend

  echo "鈴?绛夊緟鏈嶅姟鍚姩..."
  if ! wait_for_backend_healthy; then
    echo "馃洃 杩佺Щ鍚庢湇鍔″惎鍔ㄥけ璐?
    return 1
  fi

  echo "鉁?SQLite -> PostgreSQL 杩佺Щ瀹屾垚"
}



# 鍗歌浇鍔熻兘
uninstall_panel() {
  echo "馃棏锔?寮€濮嬪嵏杞介潰鏉?.."
  check_docker

  if [[ ! -f "docker-compose.yml" ]]; then
    echo "鈿狅笍 鏈壘鍒?docker-compose.yml 鏂囦欢锛屾鍦ㄤ笅杞戒互瀹屾垚鍗歌浇..."
    ask_proxy_config
    ensure_compose_urls_initialized || return 1
    DOCKER_COMPOSE_URL=$(get_docker_compose_url)
    echo "馃摗 閫夋嫨閰嶇疆鏂囦欢锛?(basename "$DOCKER_COMPOSE_URL")"
    curl -L -o docker-compose.yml "$DOCKER_COMPOSE_URL"
    echo "鉁?docker-compose.yml 涓嬭浇瀹屾垚"
  fi

  read -p "纭鍗歌浇闈㈡澘鍚楋紵姝ゆ搷浣滃皢鍋滄骞跺垹闄ゆ墍鏈夊鍣ㄥ拰鏁版嵁 (y/N): " confirm
  if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "鉂?鍙栨秷鍗歌浇"
    return 0
  fi

  echo "馃洃 鍋滄骞跺垹闄ゅ鍣ㄣ€侀暅鍍忋€佸嵎..."
  $DOCKER_CMD down --rmi all --volumes --remove-orphans
  echo "馃Ч 鍒犻櫎閰嶇疆鏂囦欢..."
  rm -f docker-compose.yml .env
  echo "鉁?鍗歌浇瀹屾垚"
}

# 涓婚€昏緫
main() {

  # 鏄剧ず浜や簰寮忚彍鍗?
  while true; do
    show_menu
    read -p "璇疯緭鍏ラ€夐」 (1-5): " choice

    case $choice in
      1)
        install_panel
        delete_self
        exit 0
        ;;
      2)
        update_panel
        delete_self
        exit 0
        ;;
      3)
        uninstall_panel
        delete_self
        exit 0
        ;;
      4)
        migrate_to_postgres
        delete_self
        exit 0
        ;;
      5)
        echo "馃憢 閫€鍑鸿剼鏈?
        delete_self
        exit 0
        ;;
      *)
        echo "鉂?鏃犳晥閫夐」锛岃杈撳叆 1-5"
        echo ""
        ;;
    esac
  done
}

# 鎵ц涓诲嚱鏁?
main
