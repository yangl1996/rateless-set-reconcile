#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "plot.pdf"

set xlabel "Overlap"
set ylabel "Fail rate"
set y2label "Overhead"
set ytics nomirror
set notitle
set yrange [0:1]
set xrange [0:500]
set y2range [0:10]
set y2tics

plot 'first.csv' u 1:3 w lines axes x1y1 title "Fail rate" lw 2 lt 3, \
     'first.csv' u 1:($4/(500.0-$1)) w lines axes x1y2 title "Overhead" lw 2 lt 4

