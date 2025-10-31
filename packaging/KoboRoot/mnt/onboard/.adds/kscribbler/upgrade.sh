ping -c 1 8.8.8.8 > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "no internet detected"
  exit 1
fi

rm -f /mnt/onboard/.kobo/KoboRoot.tgz
KSCRIBBLER_CURRENT_VERSION=$(/opt/bin/kscribbler --version)
KSCRIBBLER_CURRENT_HASH=$(wget -q -O - https://github.com/GianniBYoung/kscribbler/releases/download/$KSCRIBBLER_CURRENT_VERSION/checksums.txt)
KSCRIBBLER_LATEST_HASH=$( wget -q -O - https://github.com/GianniBYoung/kscribbler/releases/latest/download/checksums.txt)

if [ "$KSCRIBBLER_CURRENT_HASH" = "$KSCRIBBLER_LATEST_HASH" ]; then
  echo "kscribbler is already up to date"
  exit 0
fi

wget -q -O /mnt/onboard/.kobo/KoboRoot.tgz https://github.com/GianniBYoung/kscribbler/releases/latest/download/KoboRoot.tgz && \
dbus-send --system /com/kobo/nickel com.kobo.nickel.NICKEL.upgradeCheck
echo "Download complete. Please trigger a kobo sync and follow any on-screen instructions."
