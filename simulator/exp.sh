#!/bin/sh
echo "#" "$@" > data.txt
./simulator "$@" | sed -n 's/\([0-9]*.[0-9]*\) Node 1.*send rate \([0-9]*.[0-9]*\).*queue length \([0-9]*\).*/\1 \2 \3/p' >> data.txt
gnuplot time-series.gnuplot

