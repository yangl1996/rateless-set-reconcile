#!/usr/local/bin/gnuplot

set term pdf size 1.9,1.74 enhanced
set output "fig-overlap.pdf"

set size ratio 1

#set lmargin 0
set xlabel "Rate of sender 1 (s^{-1})" offset 0,0.2
set ylabel "Rate of sender 2 (s^{-1})" offset 0,0
set notitle
set yrange [0:2]
set xrange [0:2]
set xtics ("0" 0, "1000" 1, "2000" 2)
set ytics ("0" 0, "1000" 1, "2000" 2)

# Gnuplot can only smooth a curve of format y(x).
# We perform a quick transformation from (x, y) to (p, q)
#
# p = (x-y)/sqrt(2)
# q = (x+y)/sqrt(2)
#
# x = (p+q)/sqrt(2)
# y = (q-p)/sqrt(2)
# 
# and let Gnuplot do the smoothing on the p(q) domain.

set table "results2.txt"
plot "results.txt" u (($1-$2)/sqrt(2)):(($1+$2)/sqrt(2)) notitle s sbezier
unset table

set style fill pattern 7
set tics front

set style textbox opaque noborder
set label "Infeasible" at 0.5,0.5 center rotate by -45 front textcolor black boxed

plot 'results2.txt' u (($1+$2)/sqrt(2)):(($2-$1)/sqrt(2)) w filledcurves x1 notitle lc rgb "#b0b0b0", \
     'results2.txt' u (($1+$2)/sqrt(2)):(($2-$1)/sqrt(2)) w lines notitle lc rgb "#b0b0b0" lw 1 , \
     'log1.txt' u 2:3 w lines s bezier notitle lw 2 lt 1, \
     'log2.txt' u 2:3 w lines s bezier  notitle lw 2 lt 2

