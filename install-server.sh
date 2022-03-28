#!/usr/bin/env bash
# modifed from https://github.com/v2fly/fhs-install-v2ray/blob/master/install-release.sh
# You can set this variable whatever you want in shell session right before running this script by issuing:
# export TOML_PATH='/usr/local/etc/verysimple'
TOML_PATH=${TOML_PATH:-/usr/local/etc/verysimple}

curl() {
  $(type -P curl) -L -q --retry 5 --retry-delay 10 --retry-max-time 60 "$@"
}


## Demo function for processing parameters
judgment_parameters() {
  while [[ "$#" -gt '0' ]]; do
    case "$1" in
      '--remove')
        if [[ "$#" -gt '1' ]]; then
          echo 'error: Please enter the correct parameters.'
          exit 1
        fi
        REMOVE='1'
        ;;
      '--version')
        VERSION="${2:?error: Please specify the correct version.}"
        break
        ;;
      '-c' | '--check')
        CHECK='1'
        break
        ;;
      '-f' | '--force')
        FORCE='1'
        break
        ;;
      '-h' | '--help')
        HELP='1'
        break
        ;;
      '-l' | '--local')
        LOCAL_INSTALL='1'
        LOCAL_FILE="${2:?error: Please specify the correct local file.}"
        break
        ;;
      '-p' | '--proxy')
        if [[ -z "${2:?error: Please specify the proxy server address.}" ]]; then
          exit 1
        fi
        PROXY="$2"
        shift
        ;;
      *)
        echo "$0: unknown option -- -"
        exit 1
        ;;
    esac
    shift
  done
}

install_software() {
  package_name="$1"
  file_to_detect="$2"
  type -P "$file_to_detect" > /dev/null 2>&1 && return
  if ${PACKAGE_MANAGEMENT_INSTALL} "$package_name"; then
    echo "info: $package_name is installed."
  else
    echo "error: Installation of $package_name failed, please check your network."
    exit 1
  fi
}
check_if_running_as_root() {
  # If you want to run as another user, please modify $UID to be owned by this user
  if [[ "$UID" -ne '0' ]]; then
    echo "WARNING: The user currently executing this script is not root. You may encounter the insufficient privilege error."
    read -r -p "Are you sure you want to continue? [y/n] " cont_without_been_root
    if [[ x"${cont_without_been_root:0:1}" = x'y' ]]; then
      echo "Continuing the installation with current user..."
    else
      echo "Not running with root, exiting..."
      exit 1
    fi
  fi
}

identify_the_operating_system_and_architecture() {
  if [[ "$(uname)" == 'Linux' ]]; then
    case "$(uname -m)" in
      'amd64' | 'x86_64')
        MACHINE='amd64'
        ;;
      'armv8' | 'aarch64')
        MACHINE='arm64'
        ;;
      *)
        echo "error: The architecture is not supported."
        exit 1
        ;;
    esac
    if [[ ! -f '/etc/os-release' ]]; then
      echo "error: Don't use outdated Linux distributions."
      exit 1
    fi
    # Do not combine this judgment condition with the following judgment condition.
    ## Be aware of Linux distribution like Gentoo, which kernel supports switch between Systemd and OpenRC.
    ### Refer: https://github.com/v2fly/fhs-install-v2ray/issues/84#issuecomment-688574989
    if [[ -f /.dockerenv ]] || grep -q 'docker\|lxc' /proc/1/cgroup && [[ "$(type -P systemctl)" ]]; then
      true
    elif [[ -d /run/systemd/system ]] || grep -q systemd <(ls -l /sbin/init); then
      true
    else
      echo "error: Only Linux distributions using systemd are supported."
      exit 1
    fi
    if [[ "$(type -P apt)" ]]; then
      PACKAGE_MANAGEMENT_INSTALL='apt -y --no-install-recommends install'
      PACKAGE_MANAGEMENT_REMOVE='apt purge'
      package_provide_tput='ncurses-bin'
    elif [[ "$(type -P dnf)" ]]; then
      PACKAGE_MANAGEMENT_INSTALL='dnf -y install'
      PACKAGE_MANAGEMENT_REMOVE='dnf remove'
      package_provide_tput='ncurses'
    elif [[ "$(type -P yum)" ]]; then
      PACKAGE_MANAGEMENT_INSTALL='yum -y install'
      PACKAGE_MANAGEMENT_REMOVE='yum remove'
      package_provide_tput='ncurses'
    elif [[ "$(type -P zypper)" ]]; then
      PACKAGE_MANAGEMENT_INSTALL='zypper install -y --no-recommends'
      PACKAGE_MANAGEMENT_REMOVE='zypper remove'
      package_provide_tput='ncurses-utils'
    elif [[ "$(type -P pacman)" ]]; then
      PACKAGE_MANAGEMENT_INSTALL='pacman -Syu --noconfirm'
      PACKAGE_MANAGEMENT_REMOVE='pacman -Rsn'
      package_provide_tput='ncurses'
    else
      echo "error: The script does not support the package manager in this operating system."
      exit 1
    fi
  else
    echo "error: This operating system is not supported."
    exit 1
  fi
}

get_version() {
  # 0: Install or update verysimple.
  # 1: Installed or no new version of verysimple.
  # 2: Install the specified version of verysimple.
  if [[ -n "$VERSION" ]]; then
    RELEASE_VERSION="v${VERSION#v}"
    return 2
  fi
  # Determine the version number for verysimple installed from a local file
  if [[ -f '/usr/local/bin/verysimple' ]]; then
    VERSION="$(/usr/local/bin/verysimple -h 2>/dev/null | grep verysimple | awk '{print $2}')"
    CURRENT_VERSION="v${VERSION#v}"
    if [[ "$LOCAL_INSTALL" -eq '1' ]]; then
      RELEASE_VERSION="$CURRENT_VERSION"
      return
    fi
  fi
  # Get verysimple release version number
  TMP_FILE="$(mktemp)"
  if ! curl -x "${PROXY}" -sS -H "Accept: application/vnd.github.v3+json" -o "$TMP_FILE" 'https://api.github.com/repos/hahahrfool/v2ray_simple/releases/latest'; then
    "rm" "$TMP_FILE"
    echo 'error: Failed to get release list, please check your network.'
    exit 1
  fi
  RELEASE_LATEST="$(sed 'y/,/\n/' "$TMP_FILE" | grep 'tag_name' | awk -F '"' '{print $4}')"
  "rm" "$TMP_FILE"
  RELEASE_VERSION="v${RELEASE_LATEST#v}"
  # Compare verysimple version numbers
  if [[ "$RELEASE_VERSION" != "$CURRENT_VERSION" ]]; then
    RELEASE_VERSIONSION_NUMBER="${RELEASE_VERSION#v}"
    RELEASE_MAJOR_VERSION_NUMBER="${RELEASE_VERSIONSION_NUMBER%%.*}"
    RELEASE_MINOR_VERSION_NUMBER="$(echo "$RELEASE_VERSIONSION_NUMBER" | awk -F '.' '{print $2}')"
    RELEASE_MINIMUM_VERSION_NUMBER="${RELEASE_VERSIONSION_NUMBER##*.}"
    # shellcheck disable=SC2001
    CURRENT_VERSIONSION_NUMBER="$(echo "${CURRENT_VERSION#v}" | sed 's/-.*//')"
    CURRENT_MAJOR_VERSION_NUMBER="${CURRENT_VERSIONSION_NUMBER%%.*}"
    CURRENT_MINOR_VERSION_NUMBER="$(echo "$CURRENT_VERSIONSION_NUMBER" | awk -F '.' '{print $2}')"
    CURRENT_MINIMUM_VERSION_NUMBER="${CURRENT_VERSIONSION_NUMBER##*.}"
    if [[ "$RELEASE_MAJOR_VERSION_NUMBER" -gt "$CURRENT_MAJOR_VERSION_NUMBER" ]]; then
      return 0
    elif [[ "$RELEASE_MAJOR_VERSION_NUMBER" -eq "$CURRENT_MAJOR_VERSION_NUMBER" ]]; then
      if [[ "$RELEASE_MINOR_VERSION_NUMBER" -gt "$CURRENT_MINOR_VERSION_NUMBER" ]]; then
        return 0
      elif [[ "$RELEASE_MINOR_VERSION_NUMBER" -eq "$CURRENT_MINOR_VERSION_NUMBER" ]]; then
        if [[ "$RELEASE_MINIMUM_VERSION_NUMBER" -gt "$CURRENT_MINIMUM_VERSION_NUMBER" ]]; then
          return 0
        else
          return 1
        fi
      else
        return 1
      fi
    else
      return 1
    fi
  elif [[ "$RELEASE_VERSION" == "$CURRENT_VERSION" ]]; then
    return 1
  fi
}

download_verysimple() {
  DOWNLOAD_LINK="https://github.com/hahahrfool/v2ray_simple/releases/download/$RELEASE_VERSION/verysimple_linux_$MACHINE.tgz"
  echo "Downloading verysimple archive: $DOWNLOAD_LINK"
  if ! curl -x "${PROXY}" -sS -L -H 'Cache-Control: no-cache' "$DOWNLOAD_LINK" -o - | tar zxf - -C "$TMP_DIRECTORY"; then
    echo 'error: Download failed! Please check your network or try again.'
    return 1
  fi
}

download_geo_lite2() {
  DOWNLOAD_LINK="https://git.io/GeoLite2-Country.mmdb"
  echo "Downloading GeoLite2-Country: $DOWNLOAD_LINK"
  if ! curl -x "${PROXY}" -sS -L -H 'Cache-Control: no-cache' "$DOWNLOAD_LINK" -o "$TMP_DIRECTORY/GeoLite2-Country.mmdb"; then
    echo 'error: Download failed! Please check your network or try again.'
    return 1
  fi
}

install_file() {
  NAME="$1"
  if [[ "$NAME" == "verysimple" ]] ; then
    install -m 755 "${TMP_DIRECTORY}/$NAME" "/usr/local/bin/verysimple"
  fi
  if [[ "$NAME" == "GeoLite2-Country.mmdb" ]] ; then
    install -m 644 "${TMP_DIRECTORY}/$NAME" "/usr/local/etc/verysimple/GeoLite2-Country.mmdb"
  fi
}

install_verysimple() {
  # Install verysimple binary to /usr/local/bin/
  install_file verysimple
  install_file GeoLite2-Country.mmdb
  # Install verysimple configuration file to $TOML_PATH
  # shellcheck disable=SC2153
  if [[ -z "$JSONS_PATH" ]] && [[ ! -d "$TOML_PATH" ]]; then
    install -d "$TOML_PATH"
    cat << EOF >> "${TOML_PATH}/config.toml"
[[listen]]
protocol = "vless"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "0.0.0.0"
port = 4434
version = 0
insecure = true
fallback = ":80"
advancedLayer = "grpc"
path = "ohmygod_verysimple_is_very_simple"  #正常来说不宜前面再加斜杠,不过我也没试过，也许加了也能用(两端都加的情况下)

# 如需使用 Nginx、Caddy 等软件进行分流，设置的分流路径应为 /${path}/Tun

[[dial]]
protocol = "direct"
EOF
    CONFIG_NEW='1'
  fi
}

install_startup_service_file() {
  useradd -s /sbin/nologin --create-home verysimple 
	[ $? -eq 0 ] && echo "User verysimple has been added."
  echo "[Unit]
Description=verysimple, a very simple implementation of V2Ray with some innovation
Documentation=https://github.com/hahahrfool/v2ray_simple/wiki
After=network.target

[Service]
User=verysimple
CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_RAW
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_RAW
NoNewPrivileges=true
WorkingDirectory=/usr/local/etc/verysimple
ExecStart=/usr/local/bin/verysimple -c /usr/local/etc/verysimple/config.toml
Restart=on-failure
RestartPreventExitStatus=1
RestartSec=5

[Install]
WantedBy=multi-user.target" > /lib/systemd/system/verysimple-server.service
  echo "[Unit]
Description=verysimple, a very simple implementation of V2Ray with some innovation
Documentation=https://github.com/hahahrfool/v2ray_simple/wiki
After=network.target

[Service]
User=verysimple
CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_RAW
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_RAW
NoNewPrivileges=true
WorkingDirectory=/usr/local/etc/verysimple
ExecStart=/usr/local/bin/verysimple -c /usr/local/etc/verysimple/%i.toml
Restart=on-failure
RestartPreventExitStatus=1
RestartSec=5

[Install]
WantedBy=multi-user.target" > /lib/systemd/system/verysimple-server@.service
  echo "info: Systemd service files have been installed successfully!"
  systemctl daemon-reload
  SYSTEMD='1'
}

start_verysimple() {
  if [[ -f '/lib/systemd/system/verysimple-server.service' ]]; then
    if systemctl start "${VERYSIMPLE_CUSTOMIZE:-verysimple}"; then
      echo 'info: Start the verysimple service.'
    else
      echo '${red}error: Failed to start verysimple service.'
      exit 1
    fi
  fi
}

stop_verysimple() {
  VERYSIMPLE_CUSTOMIZE="$(systemctl list-units | grep 'verysimple@' | awk -F ' ' '{print $1}')"
  if [[ -z "$VERYSIMPLE_CUSTOMIZE" ]]; then
    local verysimple_daemon_to_stop='verysimple-server.service'
  else
    local verysimple_daemon_to_stop="$VERYSIMPLE_CUSTOMIZE"
  fi
  if ! systemctl stop "$verysimple_daemon_to_stop"; then
    echo 'error: Stopping the verysimple service failed.'
    exit 1
  fi
  echo 'info: Stop the verysimple service.'
}

check_update() {
  if [[ -f '/lib/systemd/system/verysimple-server.service' ]]; then
    get_version
    local get_ver_exit_code=$?
    if [[ "$get_ver_exit_code" -eq '0' ]]; then
      echo "info: Found the latest release of verysimple $RELEASE_VERSION. (Current release: $CURRENT_VERSION)"
    elif [[ "$get_ver_exit_code" -eq '1' ]]; then
      echo "info: No new version. The current version of verysimple is $CURRENT_VERSION."
    fi
    exit 0
  else
    echo 'error: verysimple is not installed.'
    exit 1
  fi
}

remove_verysimple() {
  if systemctl list-unit-files | grep -qw 'verysimple'; then
    if [[ -n "$(pidof verysimple)" ]]; then
      stop_verysimple
    fi
    if ! ("rm" -r '/usr/local/bin/verysimple' \
      '/lib/systemd/system/verysimple-server.service' \
      '/lib/systemd/system/verysimple-server@.service'); then
      echo 'error: Failed to remove verysimple.'
      exit 1
    else
      echo 'removed: /usr/local/bin/verysimple'
      echo 'removed: /lib/systemd/system/verysimple-server.service'
      echo 'removed: /lib/systemd/system/verysimple-server@.service'
      echo 'info: verysimple has been removed.'
      echo 'info: If necessary, manually delete the configuration and log files.'
      exit 0
    fi
  else
    echo 'error: verysimple is not installed.'
    exit 1
  fi
}

# Explanation of parameters in the script
show_help() {
  echo "usage: $0 [--remove | --version number | -c | -f | -h | -l | -p]"
  echo '  [-p address] [--version number | -c | -f]'
  echo '  --remove        Remove verysimple'
  echo '  --version       Install the specified version of verysimple, e.g., --version v1.1.0.2'
  echo '  -c, --check     Check if verysimple can be updated'
  echo '  -f, --force     Force installation of the latest version of verysimple'
  echo '  -h, --help      Show help'
  echo '  -l, --local     Install verysimple from a local file'
  echo '  -p, --proxy     Download through a proxy server, e.g., -p http://127.0.0.1:8118 or -p socks5://127.0.0.1:1080'
  exit 0
}


main() {
  check_if_running_as_root
  identify_the_operating_system_and_architecture
  judgment_parameters "$@"

  install_software "$package_provide_tput" 'tput'
  red=$(tput setaf 1)
  green=$(tput setaf 2)
  aoi=$(tput setaf 6)
  reset=$(tput sgr0)

  # Parameter information
  [[ "$HELP" -eq '1' ]] && show_help
  [[ "$CHECK" -eq '1' ]] && check_update
  [[ "$REMOVE" -eq '1' ]] && remove_verysimple

  # Two very important variables
  TMP_DIRECTORY="$(mktemp -d)"
  BIN_FILE="${TMP_DIRECTORY}/verysimple"

  # Install verysimple from a local file, but still need to make sure the network is available
  if [[ "$LOCAL_INSTALL" -eq '1' ]]; then
    echo 'warn: Install verysimple from a local file, but still need to make sure the network is available.'
    echo -n 'warn: Please make sure the file is valid because we cannot confirm it. (Press any key) ...'
    read -r
  else
    # Normal way
    install_software 'curl' 'curl'
    get_version
    NUMBER="$?"
    if [[ "$NUMBER" -eq '0' ]] || [[ "$FORCE" -eq '1' ]] || [[ "$NUMBER" -eq 2 ]]; then
      echo "info: Installing verysimple $RELEASE_VERSION for $(uname -m)"
      download_verysimple
      download_geo_lite2
      if [[ "$?" -eq '1' ]]; then
        "rm" -r "$TMP_DIRECTORY"
        echo "removed: $TMP_DIRECTORY"
        exit 1
      fi
    elif [[ "$NUMBER" -eq '1' ]]; then
      echo "info: No new version. The current version of verysimple is $CURRENT_VERSION ."
      exit 0
    fi
  fi

  # Determine if verysimple is running
  if systemctl list-unit-files | grep -qw 'verysimple'; then
    if [[ -n "$(pidof verysimple)" ]]; then
      stop_verysimple
      VERYSIMPLE_RUNNING='1'
    fi
  fi
  install_verysimple
  install_startup_service_file
  echo 'installed: /usr/local/bin/verysimple'
  # If the file exists, the content output of installing or updating geoip.dat and geosite.dat will not be displayed
  if [[ "$CONFIG_NEW" -eq '1' ]]; then
    echo "installed: ${TOML_PATH}/config.toml"
  fi
  if [[ "$SYSTEMD" -eq '1' ]]; then
    echo 'installed: /lib/systemd/system/verysimple-server.service'
    echo 'installed: /lib/systemd/system/verysimple-server@.service'
  fi
  "rm" -r "$TMP_DIRECTORY"
  echo "removed: $TMP_DIRECTORY"
  if [[ "$LOCAL_INSTALL" -eq '1' ]]; then
    get_version
  fi
  echo "info: verysimple $RELEASE_VERSION is installed."
  if [[ "$VERYSIMPLE_RUNNING" -eq '1' ]]; then
    start_verysimple
  else
    echo 'Please execute the command: systemctl enable verysimple-server; systemctl start verysimple-server'
  fi
}

main "$@"

