#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.4
set output "fig-soliton-sample.pdf"

set lmargin 6.2
set bmargin 2.95
set tmargin 1
set rmargin 1.2

set ylabel "P(d)" offset 1.55,0
set xlabel "d" offset 0,0.3
set key top right
set notitle
set yrange [0:0.5]
set xrange [0:50]

plot 'results.txt' i 1 u 1:2 w impulses notitle lw 2
