#!gnuplot

set term pdf size 4,2.47 enhanced
set output "time-series.pdf"

set xlabel "Time (s)"
set ylabel "Codeword rate (s^{-1})"
set y2label "Queue length"
set ytics nomirror
set notitle
set y2tics

plot 'data.txt' u 1:2 w lines axes x1y1 title "Rate" lw 2 lt 1, \
     'data.txt' u 1:3 w lines axes x1y2 title "Queue" lw 2 lt 3

