#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "trace.pdf"

set xlabel "Time (s)"
set ylabel "Codeword"
set y2label "Tx Rcvd"
set ytics nomirror
set notitle
set yrange [0:80000]
set xrange [0:500]
set y2range [0:80000]
set y2tics


plot 'log.txt' u ($1/1000.0):4 w lines axes x1y1 title "Codeword" lw 2 lt 1, \
     'log.txt' u ($1/1000.0):5 w lines axes x1y2 title "Transaction" lw 2 lt 3

