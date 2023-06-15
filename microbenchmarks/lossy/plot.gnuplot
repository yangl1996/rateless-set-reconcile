#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "plot.pdf"

set xlabel "Num Overlap"
set ylabel "Num Codewords"
set notitle

plot 'data.txt' u 1:2 w lines title "Cw to 50% loss" lw 2 lt 1, \
     'data.txt' u 1:3 w lines title "Cw to 0% loss" lw 2 lt 2, \
     'data.txt' u 1:4 w lines title "Pending Tx at 50% loss" lw 2 lt 3

