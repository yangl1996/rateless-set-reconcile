#!/bin/sh
echo "#" "$@" > data.txt
./simulator "$@"  >> data.txt
tail -n3 data.txt
gnuplot time-series.gnuplot

