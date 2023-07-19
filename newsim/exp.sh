#!/bin/sh
echo "#" "$@" > data.txt
GOMEMLIMIT=16000MiB ./simulator "$@"  >> data.txt
cat data.txt | grep '#' 
gnuplot time-series.gnuplot

