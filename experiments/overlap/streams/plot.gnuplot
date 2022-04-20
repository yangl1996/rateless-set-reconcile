#!/usr/local/bin/gnuplot

set term pdf size 2.6,1.8 enhanced
set output "fig-overlap.pdf"

set size ratio 0.666

#set lmargin 0
set xlabel "Rate of sender 1 (s^{-1})" offset 0,0.2
set ylabel "Rate of sender 2 (s^{-1})" offset 0,0
set notitle
set yrange [0:2]
set xrange [0:3]
set xtics ("0" 0, "1000" 1, "2000" 2, "3000" 3, "4000" 4)
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
#set label "Start" at 1.9,0.9122 right front textcolor rgb "dark-violet" boxed
#set label "Start" at 1.9,0.1446 right front textcolor rgb "#009e73" boxed
#set label "End" at 0.7351,0.8797 left front textcolor rgb "#009e73"
#set label "End" at 0.7351,1.0797 left front textcolor rgb "dark-violet"

plot 'results2.txt' u (($1+$2)/sqrt(2)):(($2-$1)/sqrt(2)) w filledcurves x1 notitle lc rgb "#b0b0b0", \
     'results2.txt' u (($1+$2)/sqrt(2)):(($2-$1)/sqrt(2)) w lines notitle lc rgb "#b0b0b0" lw 1 , \
     'log1.txt' u 2:3 w lines s bezier notitle lw 2 lt 1, \
     'log2.txt' u 2:3 w lines s bezier notitle lw 2 lt 2, \
     'log3.txt' u 2:3 w lines s bezier notitle lw 2 lt 3

