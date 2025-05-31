#!/bin/sh

mkdir -p /mnt/onboard/.adds/kscribbler
cfg=/mnt/onboard/.adds/kscribbler/config.env

case "$1" in
  update)
    ;;
  *)
    if [ ! -f "$cfg" ]; then
        cat << 'EOF' > $cfg
# place your api key inside of the quotes
HARDCOVER_API_TOKEN=""
EOF
    fi
    ;;
esac

chmod 600 $cfg
exit 0
