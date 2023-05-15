#!gnuplot

set term pdf size 4,2.47 enhanced
set output "overlap-dist.pdf"

set xlabel "CDF"
set ylabel "Overlap"
set notitle

plot 'overlap.txt' u ($0 * 0.01):1 w lines axes x1y1 title "Overlap" lw 2 lt 1#, \

