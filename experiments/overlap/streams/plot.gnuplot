#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.4
set output "fig-overlap.pdf"

set xlabel "r1"
set ylabel "r2"
set notitle
set yrange [0:2.0]
set xrange [0:2.0]

plot 'results.txt' u 1:2 w lines notitle lw 2, \
     'log.txt' u 2:3 w lines notitle lw 2

