#!/bin/sh
set -e

/usr/local/bin/dockpanel &
exec nginx -g 'daemon off;'
