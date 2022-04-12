#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.74 enhanced
set output "fig-overlap.pdf"

set size ratio 1

#set lmargin 0
set xlabel "Rate of sender 1 (s^{-1})" offset 0,0.2
set ylabel "Rate of sender 2 (s^{-1})" offset 1,0
set notitle
set yrange [0:2.0]
set xrange [0:2.0]

# gnuplot can only smooth a curve of format y(x).
# we perform a quick transformation from (x, y) to (p, q) and let
# gnuplot do the smoothing on p(q)
# p = (x-y)/sqrt(2)
# q = (x+y)/sqrt(2)
#
# x = (p+q)/sqrt(2)
# y = (q-p)/sqrt(2)

set table "results2.txt"
plot "results.txt" u (($1-$2)/sqrt(2)):(($1+$2)/sqrt(2)) notitle s sbezier
unset table

plot 'results2.txt' u (($1+$2)/sqrt(2)):(($2-$1)/sqrt(2)) w lines notitle lw 1, \
     'log.txt' u 2:3 w lines notitle lw 1

