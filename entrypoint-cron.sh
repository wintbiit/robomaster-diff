#!/bin/sh

if [ -z "$CRON" ]; then
  echo "CRON env is not set. Exiting..."
  exit 1
fi

echo "$CRON /usr/local/bin/robomaster-diff-job >> /var/log/cron.log 2>&1" > /etc/crontabs/root

crond -f -L /var/log/cron.log