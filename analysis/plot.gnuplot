#!/opt/homebrew/bin/gnuplot

set term pdf size 4.8,3
set output "p.pdf"
set xlabel "arrival rate"
set ylabel "decode frac"
set key bottom left

set yrange [0:1]

plot for [i=0:*] "data-calc.txt" index i using 1:3 with lines lc i dt 2 lw 1.7 notitle, \
     for [i=0:*] "data-real.txt" index i using 1:(($5 - $3) / (($6 - $4) * $1)) with lines title columnheader(1) lc i lw 1.7
