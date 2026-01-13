#!/bin/bash
# Cron job example with sentinel
# Add to crontab: 0 2 * * * /path/to/backup-job.sh

/usr/local/bin/sentinel \
    --timeout 1h \
    --alert webhook,script \
    --webhook-url "https://hooks.slack.com/services/xxx" \
    --alert-cmd "/opt/scripts/notify-oncall.sh" \
    -- /opt/scripts/backup.sh
