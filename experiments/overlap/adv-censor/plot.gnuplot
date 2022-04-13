#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.74 enhanced
set output "fig-censor.pdf"

set size ratio 1

#set lmargin 0
set xlabel "Frac Pushed" offset 0,0.2
set ylabel "Frac Decoded" offset 0,0
set notitle
set yrange [0:1]
set xrange [0.8:1.0]

set tics front

plot 'results.txt' u 1:2 w lines title "Censored", \
     'results.txt' u 1:($1+(1.0-$1)*$2) w lines title "All"

