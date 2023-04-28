#!/usr/local/bin/gnuplot

set term pdf size 3,1.85
set output "fig-overlap.pdf"

set ylabel "Abs Overhead"
set y2label "Rel Overhead"
set xlabel "Rel Fresh"
set ytics nomirror
set y2tics
set notitle

plot 'results.txt' u 2:3 w lines axes x1y1 title "Abs" , \
     'results.txt' u 2:4 w lines axes x1y2 title "Rel"
