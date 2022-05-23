#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "fig-overlap-trace.pdf"

set rmargin 7
set lmargin 8
#set lmargin 0
set xlabel "Time (s)" offset 0,0.2
set ylabel "Codeword rate (s^{-1})" offset 1,0
set y2label "Loss rate" offset -3.5,0
set ytics nomirror
set notitle
set yrange [0:2000]
set xrange [0:1000]
set y2range [0:1]
set y2tics ("0" 0, "{/Symbol g}=0.02" 0.025, "0.1" 0.1, "0.2" 0.2, "0.3" 0.3)

set arrow from second 0,0.02 to second 1000,0.02 nohead dt 2 lc 0 lw 1.5 front

plot 'log1.txt' u ($1/1000.0):($2*1000.0) w lines axes x1y1 title "Node A Codeword" lw 2 lt 1, \
     'log1.txt' u ($1/1000.0):3 w lines axes x1y2 title "Node A Loss" lw 2 lt 3

