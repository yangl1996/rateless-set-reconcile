#!/usr/local/bin/gnuplot

set term pdf size 3.8,2
set output "compare-edges.pdf"
set ylabel "Rate"
set xlabel "Edge"
set y2label "Rel. difference"
set notitle
#set yrange [0:77]
set xrange [-0.5:75.5]
unset xtics
set ytics nomirror
set y2tics nomirror
set linetype 9 lw 1 dt 2 lc "black"
set x2zeroaxis lt 9
set key outside top center maxrows 1


set style fill transparent solid 0.3 noborder

plot "combined.csv" using 0:($4-$3)/$3 title "Rel. diff." axes x1y2 with boxes, \
  "combined.csv" using 0:3 title "Real" with lines lw 2 lc rgb '#4dbeee', \
  "combined.csv" using 0:4 title "Sim" with lines lw 2 lc rgb '#77ac30'
