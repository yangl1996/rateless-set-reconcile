#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "fig-overlap-trace.pdf"

set xlabel "Time (s)"
set ylabel "Codeword rate (s^{-1})"
set y2label "Loss rate"
set ytics nomirror
set notitle
set yrange [0:2000]
set xrange [0:500]
set y2range [0:1]
set y2tics


plot 'log1.txt' u ($1/1000.0):($2*1000.0) w lines axes x1y1 title "Codeword" lw 2 lt 1, \
     'log1.txt' u ($1/1000.0):3 w lines axes x1y2 title "Loss" lw 2 lt 3

