#!/bin/sh
./folk -port 9999 & echo $! > folk.pid
sleep 1 # give the server time to start
casperjs test browser_test.js --no-colors
kill $(cat folk.pid)
rm folk.pid