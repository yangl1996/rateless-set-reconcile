#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.4
set output "fig-windowed-comparison.pdf"

set lmargin 6.2
set bmargin 2.95
set tmargin 1
set rmargin 1.7

set ylabel "Overhead" offset 1.55,0
set xlabel "k" offset 0,0.3
set key top right
set notitle
set yrange [0:1.2]
set xrange [0:250]

plot 'results.txt' u 1:($2-1.0):3 w errorlines title "Conventional" lw 2, \
     'results.txt' u 1:($4-1.0):5 w errorlines title "Windowed" lw 2
