#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "plot.pdf"

set xlabel "Cutoff (n)"
set ylabel "Fail rate"
set y2label "#codewords"
set ytics nomirror
set notitle
set yrange [0:1]
set xrange [0:100]
set y2range [0:1000]
set y2tics

plot 'data.csv' u 1:2 w lines axes x1y1 title "Fail rate" lw 2 lt 1, \
     'data.csv' u 1:3 w lines axes x1y2 title "#codewords" lw 2 lt 3

