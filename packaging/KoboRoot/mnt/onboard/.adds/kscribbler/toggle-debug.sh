KSDEBUG="/mnt/onboard/.adds/nm/kscribbler-debug"
if [ -f "$KSDEBUG" ]; then
  echo removing debug options
  rm -f $KSDEBUG
else
  echo showing debug options
  echo -e "menu_item:main:Kscribbler Init DB (no upload):cmd_output:60000:/opt/bin/kscribbler --init >> /mnt/onboard/.adds/kscribbler/kscribbler.log 2>&1" >> $KSDEBUG
  echo -e "menu_item:main:Kscribbler Mark All Quotes as Uploaded (no upload):cmd_output:60000:/opt/bin/kscribbler --mark-all-as-uploaded >> /mnt/onboard/.adds/kscribbler/kscribbler.log 2>&1" >> $KSDEBUG
fi
