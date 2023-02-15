#!/bin/sh
echo "#" "$@" > data.txt
./simulator "$@"  >> data.txt
tail -n1 data.txt
gnuplot time-series.gnuplot

