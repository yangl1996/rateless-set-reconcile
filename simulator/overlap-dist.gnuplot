#!gnuplot

set term pdf size 5,2.47 enhanced
set output "overlap-dist.pdf"

set xlabel "CDF"
set ylabel "Overlap"
set notitle
set key right outside

plot for [i=0:19] 'exp-overlap-'.i.'.csv' u 1:2 w lines axes x1y1 title 'Node '.i lw 2 lt i+1

