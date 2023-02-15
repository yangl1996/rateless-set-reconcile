#!/bin/sh
echo "#" "$@" > data.txt
./simulator "$@"  >> data.txt
gnuplot time-series.gnuplot

