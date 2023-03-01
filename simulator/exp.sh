#!/bin/sh
echo "#" "$@" > data.txt
GOMEMLIMIT=16000MiB ./simulator "$@"  >> data.txt
tail -n3 data.txt
gnuplot time-series.gnuplot

