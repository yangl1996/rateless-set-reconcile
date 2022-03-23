#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.4
set output "fig-overlap.pdf"

set lmargin 6.2
set bmargin 2.95
set tmargin 1
set rmargin 1

set ylabel "Overhead" offset 1.55,0
set xlabel "Overlap fraction" offset 0,0.3
set key top left
set notitle
set yrange [0:1.0]
set xrange [0:1]


set style fill transparent solid 0.15 # partial transparency
set style fill noborder # no separate top/bottom lines

plot 'results.txt' i 0 u 1:($2-1.0) w lines title columnheader(1) lw 2 lc rgb '#440154', \
     'results.txt' i 0 u 1:($2-1.0-$3):($2-1.0+$3) with filledcurves notitle fc rgb '#440154', \
     'results.txt' i 1 u 1:($2-1.0) w lines title columnheader(1) lw 2 lc rgb '#2c718e', \
     'results.txt' i 1 u 1:($2-1.0-$3):($2-1.0+$3) with filledcurves notitle fc rgb '#2c718e', \
     'results.txt' i 2 u 1:($2-1.0) w lines title columnheader(1) lw 2 lc rgb '#cb4679', \
     'results.txt' i 2 u 1:($2-1.0-$3):($2-1.0+$3) with filledcurves notitle fc rgb '#cb4679'
