#!/bin/sh
set -e

/usr/local/bin/dockerpilot &
exec nginx -g 'daemon off;'
